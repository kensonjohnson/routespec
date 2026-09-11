package routespec

import (
	"fmt"
	"strings"
)

func validateInfo(info Info) {
	if info.Title == "" {
		panic("routespec: document title is required")
	}
	if info.Version == "" {
		panic("routespec: document version is required")
	}
	if info.Contact != nil {
		validateContact(*info.Contact)
	}
	if info.License != nil {
		validateLicense(*info.License)
	}
	if info.ExternalDocs != nil {
		validateExternalDocs(*info.ExternalDocs, "document external documentation")
	}
	validateServers(info.Servers)
	validateTags(info.Tags)
	for name, scheme := range info.SecuritySchemes {
		if !validComponentName(name) {
			panic(fmt.Sprintf("routespec: invalid security scheme name %q", name))
		}
		validateSecurityScheme(name, scheme)
	}
}

func validateContact(contact Contact) {
	if contact.Name == "" && contact.URL == "" && contact.Email == "" {
		panic("routespec: contact must include a name, URL, or email")
	}
}

func validateLicense(license License) {
	if license.Name == "" {
		panic("routespec: license name is required")
	}
	if license.Identifier != "" && license.URL != "" {
		panic("routespec: license identifier and URL are mutually exclusive")
	}
}

func validateExternalDocs(externalDocs ExternalDocs, label string) {
	if externalDocs.URL == "" {
		panic("routespec: " + label + " URL is required")
	}
}

func validateServers(servers []Server) {
	for _, server := range servers {
		if server.URL == "" {
			panic("routespec: server URL is required")
		}
		placeholders := serverPlaceholders(server.URL)
		for placeholder := range placeholders {
			if _, exists := server.Variables[placeholder]; !exists {
				panic(fmt.Sprintf("routespec: server URL variable %q is not declared", placeholder))
			}
		}
		for name, variable := range server.Variables {
			if name == "" {
				panic("routespec: server variable name is required")
			}
			if !placeholders[name] {
				panic(fmt.Sprintf("routespec: server variable %q is not used by the URL", name))
			}
			if variable.Default == "" {
				panic(fmt.Sprintf("routespec: server variable %q default is required", name))
			}
			if len(variable.Enum) > 0 {
				found := false
				for _, value := range variable.Enum {
					if value == variable.Default {
						found = true
						break
					}
				}
				if !found {
					panic(fmt.Sprintf("routespec: server variable %q default must appear in enum", name))
				}
			}
		}
	}
}

func serverPlaceholders(url string) map[string]bool {
	placeholders := make(map[string]bool)
	for remaining := url; ; {
		start := strings.IndexByte(remaining, '{')
		if start < 0 {
			return placeholders
		}
		remaining = remaining[start+1:]
		end := strings.IndexByte(remaining, '}')
		if end < 0 {
			panic("routespec: server URL has an unterminated variable")
		}
		name := remaining[:end]
		if name == "" {
			panic("routespec: server URL variable name is required")
		}
		placeholders[name] = true
		remaining = remaining[end+1:]
	}
}

func validateTags(tags []Tag) {
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		if tag.Name == "" {
			panic("routespec: tag name is required")
		}
		if _, exists := seen[tag.Name]; exists {
			panic(fmt.Sprintf("routespec: duplicate tag %q", tag.Name))
		}
		seen[tag.Name] = struct{}{}
		if tag.ExternalDocs != nil {
			validateExternalDocs(*tag.ExternalDocs, fmt.Sprintf("tag %q external documentation", tag.Name))
		}
	}
}

func validateSecurityScheme(name string, scheme SecurityScheme) {
	switch scheme.Type {
	case "apiKey":
		if scheme.Name == "" {
			panic(fmt.Sprintf("routespec: apiKey security scheme %q name is required", name))
		}
		if scheme.In != "query" && scheme.In != "header" && scheme.In != "cookie" {
			panic(fmt.Sprintf("routespec: apiKey security scheme %q has invalid location %q", name, scheme.In))
		}
	case "http":
		if scheme.Scheme == "" {
			panic(fmt.Sprintf("routespec: http security scheme %q scheme is required", name))
		}
	case "mutualTLS":
	case "oauth2":
		validateOAuthFlows(name, scheme.Flows)
	case "openIdConnect":
		if scheme.OpenIDConnectURL == "" {
			panic(fmt.Sprintf("routespec: openIdConnect security scheme %q URL is required", name))
		}
	default:
		panic(fmt.Sprintf("routespec: security scheme %q has unsupported type %q", name, scheme.Type))
	}
}

func validateOAuthFlows(name string, flows *OAuthFlows) {
	if flows == nil {
		panic(fmt.Sprintf("routespec: oauth2 security scheme %q flows are required", name))
	}
	flowCount := 0
	for flowName, flow := range map[string]*OAuthFlow{
		"implicit":          flows.Implicit,
		"password":          flows.Password,
		"clientCredentials": flows.ClientCredentials,
		"authorizationCode": flows.AuthorizationCode,
	} {
		if flow == nil {
			continue
		}
		flowCount++
		if flow.Scopes == nil {
			panic(fmt.Sprintf("routespec: oauth2 security scheme %q %s scopes are required", name, flowName))
		}
		switch flowName {
		case "implicit":
			if flow.AuthorizationURL == "" {
				panic(fmt.Sprintf("routespec: oauth2 security scheme %q implicit authorization URL is required", name))
			}
		case "password", "clientCredentials":
			if flow.TokenURL == "" {
				panic(fmt.Sprintf("routespec: oauth2 security scheme %q %s token URL is required", name, flowName))
			}
		case "authorizationCode":
			if flow.AuthorizationURL == "" || flow.TokenURL == "" {
				panic(fmt.Sprintf("routespec: oauth2 security scheme %q authorizationCode URLs are required", name))
			}
		}
	}
	if flowCount == 0 {
		panic(fmt.Sprintf("routespec: oauth2 security scheme %q requires a flow", name))
	}
}

func validComponentName(name string) bool {
	if name == "" {
		return false
	}
	for _, character := range name {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-_", character) {
			return false
		}
	}
	return true
}

func cloneContact(contact *Contact) *Contact {
	if contact == nil {
		return nil
	}
	clone := *contact
	return &clone
}

func cloneLicense(license *License) *License {
	if license == nil {
		return nil
	}
	clone := *license
	return &clone
}

func cloneExternalDocs(externalDocs *ExternalDocs) *ExternalDocs {
	if externalDocs == nil {
		return nil
	}
	clone := *externalDocs
	return &clone
}

func cloneServers(servers []Server) []Server {
	if servers == nil {
		return nil
	}
	clone := make([]Server, len(servers))
	for index, server := range servers {
		clone[index] = server
		if server.Variables != nil {
			clone[index].Variables = make(map[string]ServerVariable, len(server.Variables))
			for name, variable := range server.Variables {
				variable.Enum = append([]string(nil), variable.Enum...)
				clone[index].Variables[name] = variable
			}
		}
	}
	return clone
}

func cloneTags(tags []Tag) []Tag {
	if tags == nil {
		return nil
	}
	clone := make([]Tag, len(tags))
	for index, tag := range tags {
		clone[index] = tag
		clone[index].ExternalDocs = cloneExternalDocs(tag.ExternalDocs)
	}
	return clone
}

func cloneSecurityScheme(scheme SecurityScheme) SecurityScheme {
	clone := scheme
	clone.Flows = cloneOAuthFlows(scheme.Flows)
	return clone
}

func cloneOAuthFlows(flows *OAuthFlows) *OAuthFlows {
	if flows == nil {
		return nil
	}
	clone := *flows
	clone.Implicit = cloneOAuthFlow(flows.Implicit)
	clone.Password = cloneOAuthFlow(flows.Password)
	clone.ClientCredentials = cloneOAuthFlow(flows.ClientCredentials)
	clone.AuthorizationCode = cloneOAuthFlow(flows.AuthorizationCode)
	return &clone
}

func cloneOAuthFlow(flow *OAuthFlow) *OAuthFlow {
	if flow == nil {
		return nil
	}
	clone := *flow
	if flow.Scopes != nil {
		clone.Scopes = make(map[string]string, len(flow.Scopes))
		for name, description := range flow.Scopes {
			clone.Scopes[name] = description
		}
	}
	return &clone
}
