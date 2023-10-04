package main

import (
	"github.com/go-faster/errors"
	"github.com/iancoleman/strcase"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
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
	Imports      []ImportInfo
	InterFaces   []SeparatedInterface
	ErrorHandler *ParsedInterfaceMethod
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
	}

	readyParsedMethods := make([]string, 0)
	switch procType {
	case EachProc:
		for _, method := range inter.Methods {
			if method.MethodName != "NewError" {
				genInfo.InterFaces = append(genInfo.InterFaces, SeparatedInterface{
					InterfaceName: method.MethodName + "Handler",
					Methods:       []ParsedInterfaceMethod{method},
				})
				readyParsedMethods = append(readyParsedMethods, method.MethodName)
			}
		}

	case TagsProc:
		tagMaps := make(map[string]*SeparatedInterface)
		existsTags := make([]string, 0)
		lo.Map(docModel.Model.Tags, func(tag *base.Tag, _ int) string {
			existsTags = append(existsTags, tag.Name)
			return tag.Name
		})
		for _, pathItem := range docModel.Model.Paths.PathItems {
			for _, op := range pathItem.GetOperations() {
				for _, tagName := range op.Tags {
					if _, ok := tagMaps[tagName]; !ok && lo.Contains(existsTags, tagName) {
						tagMaps[tagName] = &SeparatedInterface{
							InterfaceName: strcase.ToCamel(tagName) + "Service",
							Methods:       make([]ParsedInterfaceMethod, 0),
						}
					}
				}
			}
		}
		for _, pathItem := range docModel.Model.Paths.PathItems {
			for _, op := range pathItem.GetOperations() {
				for _, tagName := range op.Tags {
					for tagNameP, sepInter := range tagMaps {
						if tagNameP == tagName {
							for _, method := range inter.Methods {
								if strings.Contains(method.Comment, op.OperationId) && method.MethodName != "NewError" {
									log.Debug().Msgf("tag: %s, method: %s, opname: %s, contains %s\n", tagName, method.MethodName, op.OperationId, strings.Contains(method.Comment, op.OperationId))
									sepInter.Methods = append(sepInter.Methods, method)
									readyParsedMethods = append(readyParsedMethods, method.MethodName)
								}
							}
						}
					}
				}
			}
		}

		for _, sepInter := range tagMaps {
			genInfo.InterFaces = append(genInfo.InterFaces, *sepInter)
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
					if strings.Contains(method.Comment, op.OperationId) && method.MethodName != "NewError" {
						sepInterface.Methods = append(sepInterface.Methods, method)
						readyParsedMethods = append(readyParsedMethods, method.MethodName)
					}
				}
			}
			genInfo.InterFaces = append(genInfo.InterFaces, sepInterface)
		}
	}

	if errHandle, ok := lo.Find(inter.Methods, func(item ParsedInterfaceMethod) bool {
		return item.MethodName == "NewError"
	}); ok {
		genInfo.ErrorHandler = &errHandle
	}
	unmatchedMethods := make([]ParsedInterfaceMethod, 0)
	for _, method := range lo.Uniq(inter.Methods) {
		if !lo.Contains(readyParsedMethods, method.MethodName) && method.MethodName != "NewError" {
			unmatchedMethods = append(unmatchedMethods, method)
		}
	}
	if len(unmatchedMethods) != 0 {
		genInfo.InterFaces = append(genInfo.InterFaces, SeparatedInterface{InterfaceName: "UnmatchedMethodsHandler", Methods: unmatchedMethods})
	}
	return genInfo, nil
}
