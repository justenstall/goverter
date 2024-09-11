// Code generated by github.com/jmattheis/goverter, DO NOT EDIT.
//go:build !goverter

package generated

import dateformat "github.com/jmattheis/goverter/example/context/date-format"

type ConverterImpl struct{}

func (c *ConverterImpl) Convert(source map[string]dateformat.Input, context string) map[string]dateformat.Output {
	var mapStringExampleOutput map[string]dateformat.Output
	if source != nil {
		mapStringExampleOutput = make(map[string]dateformat.Output, len(source))
		for key, value := range source {
			mapStringExampleOutput[key] = c.exampleInputToExampleOutput(value, context)
		}
	}
	return mapStringExampleOutput
}
func (c *ConverterImpl) exampleInputToExampleOutput(source dateformat.Input, context string) dateformat.Output {
	var exampleOutput dateformat.Output
	exampleOutput.Name = source.Name
	exampleOutput.CreatedAt = dateformat.FormatTime(source.CreatedAt, context)
	return exampleOutput
}
