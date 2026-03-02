package naming

import (
	"testing"
)

func TestToGoName_SnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user_id", "UserID"},
		{"pet_name", "PetName"},
		{"created_at", "CreatedAt"},
		{"api_key", "APIKey"},
		{"some_url", "SomeURL"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToGoName_KebabCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"kebab-case-name", "KebabCaseName"},
		{"X-Request-ID", "XRequestID"},
		{"content-type", "ContentType"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToGoName_Acronyms(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"id", "ID"},
		{"url", "URL"},
		{"api", "API"},
		{"http", "HTTP"},
		{"json", "JSON"},
		{"xml", "XML"},
		{"sql", "SQL"},
		{"ssh", "SSH"},
		{"ssl", "SSL"},
		{"tls", "TLS"},
		{"tcp", "TCP"},
		{"udp", "UDP"},
		{"ip", "IP"},
		{"io", "IO"},
		{"html", "HTML"},
		{"css", "CSS"},
		{"uri", "URI"},
		{"json_api", "JSONAPI"},
		{"http_url", "HTTPURL"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToGoName_ReservedWords(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"type", "Type_"},
		{"range", "Range_"},
		{"map", "Map_"},
		{"func", "Func_"},
		{"interface", "Interface_"},
		{"struct", "Struct_"},
		{"chan", "Chan_"},
		{"go", "Go_"},
		{"select", "Select_"},
		{"case", "Case_"},
		{"default", "Default_"},
		{"return", "Return_"},
		{"var", "Var_"},
		{"const", "Const_"},
		{"import", "Import_"},
		{"package", "Package_"},
		{"defer", "Defer_"},
		{"fallthrough", "Fallthrough_"},
		{"goto", "Goto_"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToGoName_DottedNames(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"some.dotted.name", "SomeDottedName"},
		{"com.example.api", "ComExampleAPI"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToGoName_EmptyString(t *testing.T) {
	got := ToGoName("")
	if got != "Unknown" {
		t.Errorf("ToGoName(%q) = %q, want %q", "", got, "Unknown")
	}
}

func TestToGoName_AlreadyPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"PetStatus", "PetStatus"},
		{"HTMLParser", "HTMLParser"},
		{"UserName", "UserName"},
		{"MyAPI", "MyAPI"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToGoName_CamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"petStatus", "PetStatus"},
		{"firstName", "FirstName"},
		{"userID", "UserID"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToGoParamName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user_id", "userID"},
		{"PetStatus", "petStatus"},
		{"pet_name", "petName"},
		{"X-Request-ID", "xRequestID"},
		{"", "unknown"},
		{"HTTPSPort", "httpsPort"},
		{"some.dotted.name", "someDottedName"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoParamName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoParamName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNamer_RegisterName(t *testing.T) {
	n := NewNamer()

	// First registration should return the name as-is.
	if got := n.RegisterName("User"); got != "User" {
		t.Errorf("first RegisterName(User) = %q, want %q", got, "User")
	}

	// Second registration should append 2.
	if got := n.RegisterName("User"); got != "User2" {
		t.Errorf("second RegisterName(User) = %q, want %q", got, "User2")
	}

	// Third registration should append 3.
	if got := n.RegisterName("User"); got != "User3" {
		t.Errorf("third RegisterName(User) = %q, want %q", got, "User3")
	}

	// A different name should be returned as-is.
	if got := n.RegisterName("Pet"); got != "Pet" {
		t.Errorf("first RegisterName(Pet) = %q, want %q", got, "Pet")
	}
}

func TestToGoName_MixedDelimiters(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my_cool-name", "MyCoolName"},
		{"hello.world_foo", "HelloWorldFoo"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToGoName_Digits(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v2_api", "V2API"},
		{"oauth2", "Oauth2"},
		{"get200Response", "Get200Response"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
