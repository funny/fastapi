package fastapi

import (
	"reflect"
	"strings"
)

func packages(apps []*App) map[string]*packageInfo {
	result := make(map[string]*packageInfo)

	for _, app := range apps {
		for _, serviceType := range app.serviceTypes {
			pkgPath := serviceType.Type().PkgPath()
			pkg, exists := result[pkgPath]
			if !exists {
				pkg = &packageInfo{
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

type packageInfo struct {
	Path     string
	Imports  []ImportInfo
	Services []*ServiceType
	Messages []*MessageType
}

type ImportInfo struct {
	Name string
	Path string
}

func (info *packageInfo) Import(t reflect.Type) {
	if t != nil {
		pkgPath := t.PkgPath()
		typeName := t.String()
		if pkgPath != info.Path && pkgPath != "" {
			for _, info := range info.Imports {
				if info.Path == pkgPath {
					return
				}
			}
			info.Imports = append(info.Imports, ImportInfo{
				strings.Split(typeName, ".")[0],
				pkgPath,
			})
		}
	}
}

func (info *packageInfo) AddService(service *ServiceType) {
	for _, s := range info.Services {
		if s.t == service.t {
			return
		}
	}
	info.Services = append(info.Services, service)
}

func (info *packageInfo) AddMessage(message *MessageType) {
	for _, m := range info.Messages {
		if m.t == message.t {
			return
		}
	}
	info.Messages = append(info.Messages, message)
	info.Import(message.t)
}
