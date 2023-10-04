package lib

import (
	"encoding/json"
	"fmt"
	"github.com/alecthomas/kingpin/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"path"
)

var (
	Commander   = kingpin.New("OgenPostgen", "PostGen")
	OgenFolder  = Commander.Flag("ogen", "Ogen folder").Short('f').Default("api").String()
	SeparateBy  = Commander.Flag("separate", "Separate by tag/each/paths").Short('s').Default("paths").String()
	PackageName = Commander.Flag("package", "Package name").Short('p').Default("api").String()
	OutFile     = Commander.Flag("out", "Out file path").Short('o').Default("").String()
	OpenapiFile = Commander.Flag("openapi", "Openapi file path").Short('a').Default("").String()
	Verbose     = Commander.Flag("verbose", "Verbose").Short('v').Bool()
)

func Run() int {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	_ = kingpin.MustParse(Commander.Parse(os.Args[1:]))
	if *OgenFolder == "" {
		log.Error().Msg("OgenFolder is not empty in args")
		return 2
	}
	log.Debug().Msgf("OgenFolder: %s", *OgenFolder)
	interfaceDecl, parceErr := ParseInterface(path.Join(*OgenFolder, "oas_server_gen.go"), "Handler")
	if parceErr != nil {
		log.Error().Err(parceErr).Msg("failed parse ogen server file")
		return 2
	}

	var info *GenInfo
	var procType ProcessType
	switch *SeparateBy {
	case "each":
		procType = EachProc
	case "tag":
		procType = TagsProc
	case "paths":
		procType = PathsProc
	default:
		log.Error().Msgf("SeparateBy: %s  - is not supported", *SeparateBy)
		return 2
	}
	genData, procErr := ProcessOpenapi(*OpenapiFile, interfaceDecl, procType)
	if procErr != nil {
		log.Error().Err(procErr).Msg("failed process openapi file")
		return 2
	}
	info = genData
	if *Verbose {
		res, err := PrettyStruct(info)
		if err != nil {
			log.Fatal().Err(err)
		}
		fmt.Println(res)
	}
	log.Info().Msgf("write to %s", *OutFile)
	if *OutFile == "" {
		*OutFile = path.Join(*OgenFolder, "oas_postgen_services_gen.go")
	}
	generateErr := Generate(*OutFile, info, *PackageName)
	if generateErr != nil {
		log.Error().Err(generateErr).Msg("")
		return 2
	}
	return 0
}
func PrettyStruct(data interface{}) (string, error) {
	val, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}
