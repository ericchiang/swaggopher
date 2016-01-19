package spec

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/kylelemons/godebug/pretty"
)

func TestSimpleParse(t *testing.T) {

	tests := []struct {
		file string
		want Swagger
	}{
		{
			file: "testdata/petstore-minimal.json",
			want: Swagger{
				Swagger: "2.0",
				Info: &Info{
					Version:        "1.0.0",
					Title:          "Swagger Petstore",
					Description:    "A sample API that uses a petstore as an example to demonstrate features in the swagger-2.0 specification",
					TermsOfService: "http://swagger.io/terms/",
					Contact: &Contact{
						Name: "Swagger API Team",
					},
					License: &License{
						Name: "MIT",
					},
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
				Definitions: Definitions{
					"Pet": Schema{},
				},
			},
		},
	}

	for i, tt := range tests {
		func() {
			f, err := os.Open(tt.file)
			if err != nil {
				t.Error(err)
				return
			}
			defer f.Close()

			var got Swagger
			if err := json.NewDecoder(f).Decode(&got); err != nil {
				t.Error(err)
				return
			}
			if diff := pretty.Compare(got, tt.want); diff != "" {
				t.Errorf("case %d: want != got: %s", i, diff)
			}
		}()
	}
}
