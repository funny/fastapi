package fastapi

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/funny/fastbin"
)

func GenCode(app *App, apps ...*App) {
	apps = append(apps, app)

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		panic("GOPATH environment variable missing")
	}

	path, err := filepath.Abs(gopath)
	if err != nil {
		panic(err)
	}
	path = filepath.Join(path, "src")

	for _, pkg := range packages(apps) {
		saveCode(
			filepath.Join(path, pkg.Path),
			filepath.Base(pkg.Path)+".fastapi.go",
			genPackage(pkg),
		)

		for _, msg := range pkg.Messages {
			fastbin.RegisterType(msg.Type())
		}
	}
}

func saveCode(dir, filename string, code []byte) {
	filename = filepath.Join(dir, filename)
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Create file '%s' failed: %s", filename, err)
	}
	if _, err := file.Write(code); err != nil {
		log.Fatalf("Write file '%s' failed: %s", filename, err)
	}
	file.Close()
}

func genPackage(pkg *Package) (code []byte) {
	tpl := template.Must(
		template.New("fastapi").Funcs(template.FuncMap{
			"Package": func() string {
				return filepath.Base(pkg.Path)
			},
		}).Parse(appTemplate),
	)

	var bf bytes.Buffer
	err := tpl.Execute(&bf, pkg)
	if err != nil {
		log.Fatalf("Generate code failed: %s", err)
	}

	code, err = format.Source(bf.Bytes())
	if err != nil {
		fmt.Print(bf.String())
		log.Fatalf("Could't format source: %s", err)
	}

	code = bytes.Replace(code, []byte("\n\n"), []byte("\n"), -1)
	code = bytes.Replace(code, []byte("n = 0\n"), []byte("\n"), -1)
	code = bytes.Replace(code, []byte("+ 0\n"), []byte("\n"), -1)
	code, err = format.Source(code)
	if err != nil {
		fmt.Print(bf.String())
		log.Fatalf("Could't format source: %s", err)
	}
	return
}
