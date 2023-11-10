package parser

import (
	"bytes"
	"text/template"
)

func replaceTemplate(yamlContent string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("yamlTemplate").Parse(yamlContent)
	if err != nil {
		return "", err
	}

	var replacedYAML bytes.Buffer
	err = tmpl.Execute(&replacedYAML, data)
	if err != nil {
		return "", err
	}

	return replacedYAML.String(), nil
}
