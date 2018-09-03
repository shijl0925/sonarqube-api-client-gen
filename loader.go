package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type version struct {
	major byte
	minor byte
	str   string
}

func newVersion(s string) *version {
	v := &version{}
	if strings.Trim(s, " ") == "" {
		s = "0.0"
	}
	v.UnmarshalJSON([]byte(s))
	return v
}

func (v *version) String() string {
	return v.str
}

func (v *version) UnmarshalJSON(raw []byte) error {
	v.str = strings.Trim(string(raw), "\"")
	seg := strings.Split(v.str, ".")
	major, err := strconv.ParseInt(seg[0], 10, 8)
	if err != nil {
		return errors.Wrapf(err, "failed to pars major version, str - %v", v.str)
	}
	v.major = byte(major)

	if len(seg) >= 2 {
		minor, err := strconv.ParseInt(seg[1], 10, 8)
		if err != nil {
			return errors.Wrapf(err, "failed to pars minor version, str - %v", v.str)
		}
		v.minor = byte(minor)
	} else {
		v.minor = 0
	}
	return nil
}

func (v *version) lessOrEqual(ov *version) bool {
	switch {
	case v.major > ov.major:
		// 3.3, 2.2 => false
		return false
	case v.major < ov.major:
		// 1.1, 2.2 => true
		return true
	case v.minor > ov.minor:
		// 1.3, 1.2 => false
		return false
	case v.minor < ov.minor:
		// 1.1, 1.2 => true
		return true
	default:
		// 1.1, 1.1 => false
		return true
	}
}

func (v *version) greater(ov *version) bool {
	return !v.lessOrEqual(ov)
}

func (v *version) isSet() bool {
	return v.major != 0 && v.minor != 0
}

type apiDefinition struct {
	Host        string
	PackageName string
	Version     *version
	WebServices []*webService
}

func (ad *apiDefinition) ensurePackageName() {
	if ad.PackageName == "" {
		ad.PackageName = "sonarqube_client"
	}
}

type webService struct {
	PackageName string
	Path        string
	Since       version
	Description string
	Actions     []*action
}

func (ws *webService) Internal() bool {
	for _, action := range ws.Actions {
		if !action.Internal {
			return false
		}
	}
	return true
}

func (ws *webService) Deprecated() bool {
	for _, action := range ws.Actions {
		if !action.DeprecatedSince.isSet() {
			return false
		}
	}
	return true
}

func (ws *webService) ServiceName() string {
	return ws.Getter() + "Service"
}

func (ws *webService) Variable() string {
	return makeUnexported(ws.ServiceName())
}

func (ws *webService) Getter() string {
	name := strings.TrimPrefix(ws.Path, "api/")
	return makeExported(snakeToCamel(name))
}

func (ws *webService) fileName() string {
	return strings.TrimPrefix(ws.Path, "api/") + ".go"
}

type action struct {
	ServiceName        string
	Key                string
	Description        string
	Since              version
	Internal           bool
	Post               bool
	HasResponseExample bool
	DeprecatedSince    version
	Changelog          []*change
	Params             []*param
}

func (a *action) MethodName() string {
	return makeExported(snakeToCamel(a.Key))
}

func (a *action) RequestTypeName() string {
	return a.ServiceName + a.MethodName() + "Request"
}

func (a *action) ResponseTypeName() string {
	return a.ServiceName + a.MethodName() + "Response"
}

func (a *action) Deprecated() bool {
	return a.DeprecatedSince.isSet()
}

type change struct {
	Description string
	Version     string
}

func (c *change) String() string {
	return c.Version + ": " + c.Description
}

type param struct {
	Key                string
	Since              version
	Description        string
	Required           bool
	Internal           bool
	ExampleValue       string
	DeprecatedSince    version
	PossibleValues     []string
	DeprecatedKey      string
	DeprecatedKeySince version
	DefaultValue       string
	MaximumValue       int
	MinimumLength      int
	MaximumLength      int
	MaxValuesAllowed   int
}

func (p *param) ParamName() string {
	return makeExported(snakeToCamel(sanitizeItentifier(p.Key)))
}

func (p *param) Deprecated() bool {
	return p.DeprecatedSince.isSet()
}

type filter struct {
	internal   bool
	deprecated bool
	version    *version
}

func url(host string, internal bool) string {
	if internal {
		return host + "/api/webservices/list?include_internals=true"
	}
	return host + "/api/webservices/list"
}

func getTargetVersion(client *http.Client, host, version string) (string, error) {
	if version != "" {
		return version, nil
	}
	resp, err := client.Get(host + "/api/server/version")
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch server version")
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("got error response from the server, code - %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read version")
	}
	version = buf.String()

	return version, nil
}

func getDefinition(client *http.Client, host string, internal bool, version *version) (*apiDefinition, error) {
	resp, err := client.Get(url(host, internal))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch api definitions")
	}
	defer resp.Body.Close()

	def := &apiDefinition{
		PackageName: packageName,
		Host:        host,
		Version:     version,
	}
	dec := json.NewDecoder(resp.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(def); err != nil {
		return nil, errors.Wrap(err, "failed to decode response")
	}

	def.ensurePackageName()
	for _, service := range def.WebServices {
		service.PackageName = def.PackageName
		for _, action := range service.Actions {
			action.ServiceName = service.ServiceName()
		}
	}

	return def, nil
}

func filterParams(params []*param, f *filter) []*param {
	result := params[:0]
	for _, p := range params {
		if !f.deprecated && p.Deprecated() ||
			!f.internal && p.Internal ||
			p.Since.greater(f.version) {
			continue
		}
		result = append(result, p)
	}
	return result
}

func filterActions(actions []*action, f *filter) []*action {
	result := actions[:0]
	for _, action := range actions {

		if !f.deprecated && action.Deprecated() ||
			!f.internal && action.Internal ||
			action.Since.greater(f.version) {
			continue
		}

		filterParams(action.Params, f)
		result = append(result, action)
	}
	return result
}

func filterDefinition(def *apiDefinition, f *filter) *apiDefinition {
	wss := def.WebServices[:0]
	for _, ws := range def.WebServices {

		if !f.deprecated && ws.Deprecated() ||
			!f.internal && ws.Internal() ||
			ws.Since.greater(f.version) {
			continue
		}

		filterActions(ws.Actions, f)

		wss = append(wss, ws)
	}
	def.WebServices = wss
	return def
}

func loadAPI(client *http.Client, host string, deprecated bool, internal bool, version string) (*apiDefinition, error) {
	if client == nil {
		client = http.DefaultClient
	}

	version, err := getTargetVersion(client, host, version)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve target version")
	}
	parsedVersion := newVersion(version)

	def, err := getDefinition(client, host, internal, parsedVersion)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load definition")
	}

	filterDefinition(def, &filter{
		deprecated: deprecated,
		internal:   internal,
		version:    parsedVersion,
	})

	return def, nil
}