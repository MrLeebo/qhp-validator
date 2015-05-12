package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	js "github.com/xeipuuv/gojsonschema"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <plans_schema.json>\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	validator, err := NewValidator(flag.Arg(0))
	if err != nil {
		log.Fatalln("new validator:", err.Error())
	}

	http.Handle("/", validator)
	http.HandleFunc("/schema/", func(w http.ResponseWriter, r *http.Request) {
		var fn string
		switch typ := r.URL.Path[len("/schema/"):]; typ {
		case "plans":
			fn = flag.Arg(0)
		case "providers":
			fn = flag.Arg(1)
		case "drugs":
			fn = flag.Arg(2)
		default:
			http.Error(w, http.StatusText(404), 404)
			return
		}
		http.ServeFile(w, r, fn)
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type Validator struct {
	schemas map[string]*js.Schema
}

func NewValidator(plansSchemaPath string) (*Validator, error) {
	v := new(Validator)
	v.schemas = make(map[string]*js.Schema)
	var err error
	v.schemas["plans"], err = js.NewSchema(js.NewReferenceLoader("file://" + abs(plansSchemaPath)))
	return v, err
}

func (v *Validator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		errors []string
		valid  bool
	)
	if r.Method == "POST" {
		// Validate JSON
		schema, ok := v.schemas[r.FormValue("doc-type")]
		if !ok {
			errors = []string{fmt.Sprintf("This document type schema not yet implemented: %q", r.FormValue("doc-type"))}
			goto render
		}
		loader := js.NewStringLoader(r.FormValue("json"))
		result, err := schema.Validate(loader)
		if err != nil {
			errors = []string{
				"JSON is not well-formed: " + err.Error(),
			}
		} else {
			if result.Valid() {
				valid = true
			} else {
				for _, err := range result.Errors() {
					errors = append(errors, err.String())
				}
			}
		}
	}
render:
	t := template.Must(template.New("index").Parse(`
<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <title>QHP provider and formulary JSON Schema validator</title>
		<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.4/css/bootstrap.min.css">
		<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.4/css/bootstrap-theme.min.css">
		<script src="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.4/js/bootstrap.min.js"></script>
    </head>
    <body>
		<div class="container">
			<h1>QHP provider and formulary JSON Schema validator</h1>
			<p><a href="https://github.com/CMSgov/QHP-provider-formulary-APIs">Click here for more information.</a></p>
			{{if .Errors}}
				<div class="text-danger">
					<p>This document is <b>not valid</b>:</p>
					<ul>
					{{range .Errors}}
						<li>{{.}}
					{{end}}
					</ul>
				</div>
			{{end}}
			{{if .Valid}}
				<div class="text-success">
					<p>This document is <b>valid</b>.</p>
				</div>
			{{end}}
			{{if eq .Method "POST"}}
			<pre>{{.JSON}}</pre>
			<hr>
			{{end}}
			<form method="post">
				<div class="form-group">
					<label for="docType">JSON document type</label>
					<select class="form-control" id="docType" name="doc-type" aria-describedby="docTypeHelp">
						<option value="plans" selected>Plans</option>
						<option value="providers" disabled>Providers</option>
						<option value="drugs" disabled>Drugs</option>
					</select>
					<span id="docTypeHelp" class="help-block">The type of JSON document to be validated.</span>
				</div>
				<div class="form-group">
					<label for="json">JSON</label>
					<textarea class="form-control" rows="5" id="json" name="json" aria-describedby="helpBlock"></textarea>
					<span id="helpBlock" class="help-block">Paste in your JSON here.</span>
				</div>
				<button type="submit" class="btn btn-default">Validate</button>
			</form>
			<hr>
			<footer>
				<p>Dump schemas:
				<ul>
					<li><a href="/schema/plans">Plans</a>
					<li><a href="/schema/providers">Providers</a>
					<li><a href="/schema/drugs">Drugs</a>
				</ul>
			</footer>
		</div>
    </body>
</html>`))
	if err := t.Execute(w, struct {
		Errors []string
		Valid  bool
		Method string
		JSON   string
	}{
		errors,
		valid,
		r.Method,
		r.FormValue("json"),
	}); err != nil {
		log.Printf("rendering html: %v", err)
		http.Error(w, http.StatusText(500), 500)
	}
}

func abs(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return p
}
