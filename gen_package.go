package fastapi

import (
	"reflect"
	"strings"
)

func packages(apps []*App) map[string]*Package {
	result := make(map[string]*Package)

	for _, app := range apps {
		for _, serviceType := range app.serviceTypes {
			pkgPath := serviceType.Type().PkgPath()
			pkg, exists := result[pkgPath]
			if !exists {
				pkg = &Package{
					Path: pkgPath,
				}
				result[pkgPath] = pkg
			}

			pkg.AddService(serviceType)

			for _, message := range serviceType.requests {
				pkg.AddMessage(message)
			}

			for _, message := range serviceType.responses {
				pkg.AddMessage(message)
			}
		}
	}

	return result
}

type Package struct {
	Path     string
	Imports  []importInfo
	Services []*ServiceType
	Messages []*MessageType
}

type importInfo struct {
	Name string
	Path string
}

func (info *Package) Import(t reflect.Type) {
	if t != nil {
		pkgPath := t.PkgPath()
		typeName := t.String()
		if pkgPath != info.Path && pkgPath != "" {
			for _, info := range info.Imports {
				if info.Path == pkgPath {
					return
				}
			}
			info.Imports = append(info.Imports, importInfo{
				strings.Split(typeName, ".")[0],
				pkgPath,
			})
		}
	}
}

func (info *Package) AddService(service *ServiceType) {
	for _, s := range info.Services {
		if s.t == service.t {
			return
		}
	}
	info.Services = append(info.Services, service)
	info.Import(service.sessionType.Elem())
}

func (info *Package) AddMessage(message *MessageType) {
	for _, m := range info.Messages {
		if m.t == message.t {
			return
		}
	}
	info.Messages = append(info.Messages, message)
	info.Import(message.t)
}
