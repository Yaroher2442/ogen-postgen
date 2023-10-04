package lib

import (
	"fmt"
	"github.com/samber/lo"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

type ParsedInterfaceMethod struct {
	Comment            string
	MethodName         string
	Params             string
	ParamsWithoutTypes string
	Returns            string
}

type ParsedInterface struct {
	InterfaceName string

	Imports []ImportInfo
	Methods []ParsedInterfaceMethod
}

// ImportInfo Struct для хранения информации об импорте
type ImportInfo struct {
	PackagePath string
	PackageName string
	Alias       string
}

// ParseInterface парсит интерфейс в файле и возвращает информацию о его методах
func ParseInterface(filePath string, interfaceName string) (*ParsedInterface, error) {
	// Создаем токен-файл для хранения информации о файле
	fset := token.NewFileSet()

	// Парсим go-файл
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Ищем нужный интерфейс по имени
	var interfaceDecl *ast.InterfaceType
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Name.Name == interfaceName {
					if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
						interfaceDecl = interfaceType
						break
					}
				}
			}
		}
	}
	// Если интерфейс не найден, возвращаем ошибку
	if interfaceDecl == nil {
		return nil, fmt.Errorf("interface %s not found", interfaceName)
	}

	// Мапа для хранения информации об импортах
	imports := make(map[string]ImportInfo)

	// Обходим импорты в файле и записываем информацию об импорте в мапу
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		name := ""
		alias := ""

		if imp.Name != nil {
			if imp.Name.Name != "_" {
				name = imp.Name.Name
			}
			alias = imp.Name.Name
		} else {
			parts := strings.Split(path, "/")
			name = parts[len(parts)-1]
		}

		// TODO: возможно ключом нужно делать не path,
		// непонятно что является ключом при получении из imports в getTypeName (t.X.(*ast.Ident) это что)
		imports[path] = ImportInfo{
			PackagePath: path,
			PackageName: name,
			Alias:       alias,
		}
	}

	out := &ParsedInterface{
		InterfaceName: interfaceName,
		Methods:       make([]ParsedInterfaceMethod, 0, len(interfaceDecl.Methods.List)),
		Imports:       lo.Values(imports),
	}
	for _, method := range interfaceDecl.Methods.List {
		methodName := method.Names[0].Name
		params := ""
		paramsWithoutTypes := ""
		returns := ""
		comments := ""

		if funcType, ok := method.Type.(*ast.FuncType); ok {
			if funcType.Params != nil {
				for _, param := range funcType.Params.List {
					paramType := getTypeName(param.Type, imports)
					paramNames := getParamNames(param)
					if len(paramNames) > 0 {
						params += strings.Join(paramNames, ", ") + " " + paramType + ", "
					} else {
						params += paramType + ", "
					}
					paramsWithoutTypes += strings.Join(paramNames, ", ") + ", "
				}
				params = strings.TrimRight(params, ", ")
				paramsWithoutTypes = strings.TrimRight(paramsWithoutTypes, ", ")
			}

			if funcType.Results != nil {
				for _, result := range funcType.Results.List {

					resultType := getTypeName(result.Type, imports)
					resultNames := getResultNames(result)
					if len(resultNames) > 0 {
						returns += strings.Join(resultNames, ", ") + " " + resultType + ", "
					} else {
						returns += resultType + ", "
					}
				}
				returns = strings.TrimRight(returns, ", ")
			}
		}
		ast.Inspect(method, func(node ast.Node) bool {
			commentGroup, ok := node.(*ast.CommentGroup)
			if !ok {
				return true
			}
			comments = commentGroup.Text()
			return true
		})

		out.Methods = append(out.Methods, ParsedInterfaceMethod{
			Comment:            comments,
			MethodName:         methodName,
			Params:             params,
			ParamsWithoutTypes: paramsWithoutTypes,
			Returns:            returns,
		})
	}

	return out, nil
}

// Функция для получения имени типа с учетом указателя и импортов
func getTypeName(expr ast.Expr, imports map[string]ImportInfo) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if pkgName, ok := t.X.(*ast.Ident); ok {
			if info, ok := imports[pkgName.Name]; ok {
				if info.Alias != "" {
					return fmt.Sprintf("%s.%s", info.Alias, t.Sel.Name)
				}
				return fmt.Sprintf("%s.%s", info.PackageName, t.Sel.Name)
			}
		}
		return fmt.Sprintf("%s.%s", t.X, t.Sel.Name)
	case *ast.StarExpr:
		return "*" + getTypeName(t.X, imports)
	case *ast.ArrayType:
		return "[]" + getTypeName(t.Elt, imports)
	default:
		return ""
	}
}

// Функция для получения имен входных параметров
func getParamNames(field *ast.Field) []string {
	var names []string
	for _, name := range field.Names {
		names = append(names, name.Name)
	}
	if len(names) == 0 {
		names = append(names, "_")
	}
	return names
}

// Функция для получения имен выходных параметров
func getResultNames(field *ast.Field) []string {
	var names []string
	for _, name := range field.Names {
		names = append(names, name.Name)
	}
	return names
}
