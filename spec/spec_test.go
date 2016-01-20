package spec

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/kylelemons/godebug/pretty"
)

func TestSimpleParse(t *testing.T) {

	petstoreMin := Swagger{
		Swagger: "2.0",
		Info: &Info{
			Version:        "1.0.0",
			Title:          "Swagger Petstore",
			Description:    "A sample API that uses a petstore as an example to demonstrate features in the swagger-2.0 specification",
			TermsOfService: "http://swagger.io/terms/",
			Contact:        &Contact{Name: "Swagger API Team"},
			License:        &License{Name: "MIT"},
		},
		Host:     "petstore.swagger.io",
		BasePath: "/api",
		Schemes:  []string{"http"},
		Consumes: []string{"application/json"},
		Produces: []string{"application/json"},
		Paths: Paths{
			"/pets": PathItem{
				Get: &Operation{
					Description: "Returns all pets from the system that the user has access to",
					Produces:    []string{"application/json"},
					Responses: Responses{
						"200": {
							Description: "A list of pets.",
							Schema:      &Schema{},
						},
					},
				},
			},
		},
		Definitions: Definitions{"Pet": Schema{}},
	}

	tests := []struct {
		file      string
		want      Swagger
		unmarshal func([]byte, interface{}) error
	}{
		{
			file:      "testdata/petstore-minimal.json",
			want:      petstoreMin,
			unmarshal: json.Unmarshal,
		},
		{
			file:      "testdata/petstore-minimal.yaml",
			want:      petstoreMin,
			unmarshal: yaml.Unmarshal,
		},
	}

	for i, tt := range tests {
		func() {
			data, err := ioutil.ReadFile(tt.file)
			if err != nil {
				t.Error(err)
				return
			}

			var got Swagger
			if err := tt.unmarshal(data, &got); err != nil {
				t.Errorf("failed to parse %s: %v", tt.file, err)
				return
			}
			if diff := pretty.Compare(got, tt.want); diff != "" {
				t.Errorf("case %d: want != got: %s", i, diff)
			}
		}()
	}
}
