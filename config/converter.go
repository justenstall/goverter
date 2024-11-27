package config

import (
	"fmt"
	"go/types"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jmattheis/goverter/enum"
	"github.com/jmattheis/goverter/method"
	"github.com/jmattheis/goverter/pkgload"
)

const (
	configExtend = "extend"
)

type Format string

const (
	FormatStruct   Format = "struct"
	FormatVariable Format = "assign-variable"
	FormatFunction Format = "function"
	FormatMethod   Format = "method"
)

var DefaultCommon = Common{
	Enum:            enum.Config{Enabled: true},
	ArgContextRegex: regexp.MustCompile("^ctx|^context"),
}

var DefaultConfigInterface = ConverterConfig{
	OutputFile:        "./generated/generated.go",
	OutputPackageName: "generated",
	Common:            DefaultCommon,
	OutputFormat:      FormatStruct,
}

var DefaultConfigVariables = ConverterConfig{
	OutputFormat: FormatVariable,
	Common:       DefaultCommon,
}

type Converter struct {
	ConverterConfig
	Package  string
	FileName string
	typ      types.Type
	Methods  []*Method

	Location string
}

func (c *Converter) typeForMethod() types.Type {
	if c.OutputFormat == FormatFunction {
		return nil
	}
	return c.typ
}

func (c *Converter) requireStruct() error {
	if c.OutputFormat == FormatStruct {
		return nil
	}
	return fmt.Errorf("not allowed when using goverter:variables")
}

func (c *Converter) IDString() string {
	if c.typ == nil {
		return "var definition"
	}
	return c.typ.String()
}

type ConverterConfig struct {
	Common
	Name              string
	OutputRaw         []string
	OutputFile        string
	OutputPackagePath string
	OutputPackageName string
	OutputFormat      Format
	OutputReceiver    string
	Extend            []*method.Definition
	Comments          []string
}

func (conf *ConverterConfig) PackageID() string {
	if conf.OutputPackageName == "" {
		return conf.OutputPackagePath
	}
	return conf.OutputPackagePath + ":" + conf.OutputPackageName
}

func defaultOutputFile(name string) string {
	f := filepath.Base(name)
	ext := filepath.Ext(f)
	return strings.TrimSuffix(f, ext) + ".gen" + ext
}

func parseConverter(ctx *context, rawConverter *RawConverter, global RawLines) (*Converter, error) {
	c, err := initConverter(ctx.Loader, rawConverter)
	if err != nil {
		return nil, err
	}

	if err := parseConverterLines(ctx, c, "global", global); err != nil {
		return nil, err
	}
	if err := parseConverterLines(ctx, c, c.IDString(), rawConverter.Converter); err != nil {
		return nil, err
	}

	err = parseMethods(ctx, rawConverter, c)
	return c, err
}

func initConverter(loader *pkgload.PackageLoader, rawConverter *RawConverter) (*Converter, error) {
	c := &Converter{
		FileName: rawConverter.FileName,
		Package:  rawConverter.PackagePath,
		Location: rawConverter.Converter.Location,
	}

	if rawConverter.InterfaceName != "" {
		c.ConverterConfig = DefaultConfigInterface
		v, err := loader.GetOneRaw(c.Package, rawConverter.InterfaceName)
		if err != nil {
			return nil, err
		}

		c.typ = v.Type()
		c.Name = rawConverter.InterfaceName + "Impl"
		return c, nil
	}

	c.ConverterConfig = DefaultConfigVariables
	c.OutputFile = defaultOutputFile(rawConverter.FileName)
	c.OutputPackageName = rawConverter.PackageName
	c.OutputPackagePath = rawConverter.PackagePath
	return c, nil
}

func parseConverterLines(ctx *context, c *Converter, source string, raw RawLines) error {
	for _, value := range raw.Lines {
		if err := parseConverterLine(ctx, c, value); err != nil {
			return formatLineError(raw, source, value, err)
		}
	}

	return nil
}

func parseConverterLine(ctx *context, c *Converter, value string) (err error) {
	cmd, rest := parseCommand(value)
	switch cmd {
	case "converter", "variables":
		// only a marker interface
	case "name":
		if err = c.requireStruct(); err != nil {
			return err
		}
		c.Name, err = parseString(rest)
	case "output:raw":
		c.OutputRaw = append(c.OutputRaw, rest)
	case "output:file":
		c.OutputFile, err = parseString(rest)
	case "output:format":
		if len(c.Extend) != 0 {
			return fmt.Errorf("Cannot change output:format after extend functions have been added.\nMove the extend below the output:format setting.")
		}
		err = parseConverterFormatLine(c, rest)
		if err != nil {
			return err
		}
	case "output:package":
		c.OutputPackageName = ""
		var pkg string
		pkg, err = parseString(rest)

		parts := strings.SplitN(pkg, ":", 2)
		switch len(parts) {
		case 2:
			c.OutputPackageName = parts[1]
			fallthrough
		case 1:
			c.OutputPackagePath = parts[0]
		}
	case "struct:comment":
		if err = c.requireStruct(); err != nil {
			return err
		}
		c.Comments = append(c.Comments, rest)
	case "enum:exclude":
		var pattern enum.IDPattern
		pattern, err = parseIDPattern(c.Package, rest)
		c.Enum.Excludes = append(c.Enum.Excludes, pattern)
	case configExtend:
		for _, name := range strings.Fields(rest) {
			opts := &method.ParseOpts{
				ErrorPrefix:       "error parsing type",
				OutputPackagePath: c.OutputPackagePath,
				Converter:         c.typeForMethod(),
				Params:            method.ParamsRequired,
				ContextMatch:      c.ArgContextRegex,
			}
			var defs []*method.Definition
			defs, err = ctx.Loader.GetMatching(c.Package, name, opts)
			if err != nil {
				break
			}
			c.Extend = append(c.Extend, defs...)
		}
	default:
		_, err = parseCommon(&c.Common, cmd, rest)
	}
	return err
}

func parseConverterFormatLine(c *Converter, rest string) (err error) {
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return nil
	}

	c.OutputFormat, err = parseEnum("format", false, fields[0], FormatFunction, FormatStruct, FormatVariable, FormatMethod)
	if err != nil {
		return err
	}

	switch c.OutputFormat {
	case FormatFunction, FormatStruct:
		if len(fields) != 1 {
			return fmt.Errorf("invalid %s value: expected one value but got %d: %s", "format", len(fields), fields)
		}
	case FormatVariable:
		if len(fields) != 1 {
			return fmt.Errorf("invalid %s value: expected one value but got %d: %s", "format", len(fields), fields)
		}
		if c.typ == nil && c.OutputFormat != FormatVariable {
			return fmt.Errorf("unsupported format for goverter:variables")
		}
		if c.typ != nil && c.OutputFormat == FormatVariable {
			return fmt.Errorf("unsupported format for goverter:converter")
		}
	case FormatMethod:
		if len(fields) != 2 {
			return fmt.Errorf("invalid %s value: expected two values but got %d: %s", "format", len(fields), fields)
		}
		// Set the method receiver
		c.OutputReceiver = fields[1]
	}

	return nil
}
