package main

import (
	"bytes"
	"fmt"
	"giruno/gitlab"
	"giruno/internals"
	"text/template"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty/gocty"
)

func main() {
	tmpl, err := template.
		New("driver_config").
		Funcs(template.FuncMap{
			"hcl": func(v interface{}) (string, error) {
				valTy, err := gocty.ImpliedType(v)
				if err != nil {
					return "", err
				}

				val, err := gocty.ToCtyValue(v, valTy)
				if err != nil {
					// This should never happen, since we should always be able
					// to decode into the implied type.
					panic(fmt.Sprintf("failed to encode %T as %#v: %s", v, valTy, err))
				}

				str := string(hclwrite.TokensForValue(val).Bytes())
				//fmt.Println(str)
				return str, nil
			},
		}).
		Parse(`
image = "{{.Service.Name}}"
entrypoint = {{.Service.Entrypoint | hcl}}
{{with .Service.Command -}}
{{if gt (len .) 0 -}}
command = "{{index . 0}}"
args = {{slice . 1 | hcl}}
{{end -}}
{{end -}}
{{with .Auth -}}
auth = {
	username = "{{.Username}}"
	password = "{{.Password}}"
}
{{end -}}`)
	if err != nil {
		panic(err)
	}
	driver_config_hcl := new(bytes.Buffer)
	err = tmpl.Execute(driver_config_hcl, map[string]interface{}{
		"Service": gitlab.JobService{
			Name:       "registry.gitlab.com/gitlab-org/cluster-integration/auto-deploy-image",
			Entrypoint: []string{"bash", "-c"},
			Command:    []string{"sleep", "infinity"},
		},
		"Auth": internals.RegistryAuth{
			Username: "gitlab+deploy-token-1",
			Password: "xxxxxxxxxxxxxxxxxxxx",
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(driver_config_hcl.String())
}
