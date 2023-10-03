package main

import (
	"github.com/go-faster/errors"
	"github.com/iancoleman/strcase"
	"github.com/pb33f/libopenapi"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"os"
	"strings"
)

type ProcessType string

const (
	EachProc  ProcessType = "each"
	TagsProc  ProcessType = "tags"
	PathsProc ProcessType = "paths"
)

type SeparatedInterface struct {
	InterfaceName string
	Methods       []ParsedInterfaceMethod
}

type GenInfo struct {
	Imports    []ImportInfo
	InterFaces []SeparatedInterface
	UnMatched  SeparatedInterface
}

func ProcessOpenapi(filePath string, inter *ParsedInterface, procType ProcessType) (*GenInfo, error) {
	fileData, readErr := os.ReadFile(filePath)
	if readErr != nil {
		return nil, readErr
	}
	// create a new document from specification bytes
	document, docParceErr := libopenapi.NewDocument(fileData)
	if docParceErr != nil {
		return nil, docParceErr
	}
	docModel, buildModelErr := document.BuildV3Model()
	if buildModelErr != nil {
		for i := range buildModelErr {
			log.Debug().Msgf("error: %e\n", buildModelErr[i])
			return nil, errors.New("cant parse model")
		}
	}
	genInfo := &GenInfo{
		Imports:    inter.Imports,
		InterFaces: make([]SeparatedInterface, 0),
		UnMatched: SeparatedInterface{
			InterfaceName: "UnmatchedMethodsHandler",
			Methods:       make([]ParsedInterfaceMethod, 0),
		},
	}
	readyParsedMethods := make([]string, 0)
	switch procType {
	case EachProc:
		for _, method := range inter.Methods {
			genInfo.InterFaces = append(genInfo.InterFaces, SeparatedInterface{
				InterfaceName: method.MethodName + "Handler",
				Methods:       []ParsedInterfaceMethod{method},
			})
		}

	case TagsProc:
		tagMaps := make(map[string]SeparatedInterface)
		for pathName, pathItem := range docModel.Model.Paths.PathItems {
			for _, op := range pathItem.GetOperations() {
				for _, tagName := range op.Tags {
					if _, ok := tagMaps[tagName]; !ok {
						tagMaps[tagName] = SeparatedInterface{
							InterfaceName: strings.Join(lo.Map(strings.Split(pathName, "/"), func(item string, index int) string {
								return strcase.ToCamel(item)
							}), "") + "ServiceTag",
							Methods: make([]ParsedInterfaceMethod, 0),
						}
					}
					for _, method := range inter.Methods {
						if strings.Contains(method.Comment, op.OperationId) || method.MethodName == "NewError" {
							sepInter := tagMaps[tagName]
							sepInter.Methods = append(sepInter.Methods, method)
							readyParsedMethods = append(readyParsedMethods, method.MethodName)
						}
					}
					if item, ok := lo.Find(inter.Methods, func(item ParsedInterfaceMethod) bool {
						return item.MethodName == "NewError"
					}); ok {
						sepInter := tagMaps[tagName]
						sepInter.Methods = append(sepInter.Methods, item)
					}
				}
			}
		}
		for _, sepInter := range tagMaps {
			genInfo.InterFaces = append(genInfo.InterFaces, sepInter)
		}

	case PathsProc:
		for pathName, pathItem := range docModel.Model.Paths.PathItems {
			sepInterface := SeparatedInterface{
				InterfaceName: strings.Join(lo.Map(strings.Split(pathName, "/"), func(item string, index int) string {
					return strcase.ToCamel(item)
				}), "") + "Service",
				Methods: make([]ParsedInterfaceMethod, 0),
			}
			for _, op := range pathItem.GetOperations() {
				for _, method := range inter.Methods {
					if strings.Contains(method.Comment, op.OperationId) {
						sepInterface.Methods = append(sepInterface.Methods, method)
						readyParsedMethods = append(readyParsedMethods, method.MethodName)
					}
				}

			}
			if item, ok := lo.Find(inter.Methods, func(item ParsedInterfaceMethod) bool {
				return item.MethodName == "NewError"
			}); ok {
				sepInterface.Methods = append(sepInterface.Methods, item)
			}
			genInfo.InterFaces = append(genInfo.InterFaces, sepInterface)
		}
	}

	for _, method := range lo.Uniq(inter.Methods) {
		if !lo.Contains(readyParsedMethods, method.MethodName) {
			genInfo.UnMatched.Methods = append(genInfo.UnMatched.Methods, method)
		}
	}
	return genInfo, nil
}
