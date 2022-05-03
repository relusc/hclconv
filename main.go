package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	hcljson "github.com/hashicorp/hcl/v2/json"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

func main() {
	in := flag.String("in", "", "Path to input file")
	out := flag.String("out", "", "Path to output file")
	format := flag.Bool("format", false, "Format generated HCL content")
	flag.Parse()

	// Abort if input or output file are missing
	if *out == "" || *in == "" {
		fmt.Fprintln(os.Stderr, "At least one parameter is missing, please set both '--in' and '--out' flags")
		os.Exit(1)
	}

	var con string
	var err error
	if strings.HasSuffix(*in, ".json") {
		con, err = hclconv(*in)
	} else {
		con, err = jsonconv(*in)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Write to file
	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	_, err = f.WriteString(con)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	f.Sync()

	if strings.HasSuffix(*in, ".json") && *format {
		// do format by using 'terraform' CLI
		cmd := exec.Command("terraform", "fmt", *out)

		err := cmd.Run()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

// Convert content to HCL
func hclconv(filename string) (string, error) {
	in, hclDiags := hcljson.ParseFile(filename)
	if hclDiags != nil {
		return "", fmt.Errorf("unable to parse JSON: %s", hclDiags)
	}

	var s hcl.BodySchema

	attr, hclDiags := in.Body.JustAttributes()
	if hclDiags != nil {
		return "", fmt.Errorf("unable to get attributes of HCL file: %s", hclDiags)
	}

	for _, at := range attr {
		s.Attributes = append(s.Attributes, hcl.AttributeSchema{Name: at.Name})
	}

	body, hclDiags := in.Body.Content(&s)
	if hclDiags != nil {
		return "", fmt.Errorf("unable to verify HCL content: %s", hclDiags)
	}

	var attributeNames []string
	for attrName := range attr {
		attributeNames = append(attributeNames, attrName)
	}

	var content string

	for _, name := range attributeNames {
		var b bytes.Buffer
		var p cty.Path

		attrVal, hclDiags := body.Attributes[name].Expr.Value(nil)
		if hclDiags != nil {
			return "", fmt.Errorf("unable to get value of HCL attribute: %s", hclDiags)
		}

		err := marshal(attrVal, attrVal.Type(), p, &b, false, false)
		if err != nil {
			return "", fmt.Errorf("error while marshalling JSON value to HCL: %s", err)
		}

		line := fmt.Sprintf("%s = %s", name, string(b.Bytes()))
		content += line
	}

	return content, nil
}

// Convert content to JSON
func jsonconv(filename string) (string, error) {
	in, hclDiags := hclparse.NewParser().ParseHCLFile(filename)
	if hclDiags != nil {
		return "", fmt.Errorf("unable to parse HCL: %s", hclDiags)
	}

	var s hcl.BodySchema

	attr, hclDiags := in.Body.JustAttributes()
	if hclDiags != nil {
		return "", fmt.Errorf("unable to get attributes of HCL file: %s", hclDiags)
	}

	for _, at := range attr {
		s.Attributes = append(s.Attributes, hcl.AttributeSchema{Name: at.Name})
	}

	body, hclDiags := in.Body.Content(&s)
	if hclDiags != nil {
		return "", fmt.Errorf("unable to verify HCL content: %s", hclDiags)
	}

	content := "{"

	var attributeNames []string
	for attrName := range attr {
		attributeNames = append(attributeNames, attrName)
	}

	for i, name := range attributeNames {
		attrVal, hclDiags := body.Attributes[name].Expr.Value(nil)
		if hclDiags != nil {
			return "", fmt.Errorf("unable to get value of HCL attribute: %s", hclDiags)
		}

		j, err := ctyjson.Marshal(attrVal, attrVal.Type())
		if err != nil {
			return "", fmt.Errorf("error while marshalling HCL value to JSON: %s", err)
		}

		line := fmt.Sprintf("\"%s\":%s", name, string(j))
		content += line

		// Add comma only if not last line / last attribute
		if i != len(attributeNames)-1 {
			content += ","
		}
	}

	content += "}"

	var out bytes.Buffer
	err := json.Indent(&out, []byte(content), "", "  ")
	if err != nil {
		return "", fmt.Errorf("error while formatting JSON content: %s", err)
	}

	return out.String(), nil
}

// Helper function for marshalling HCL values
// this is heavily inherited from https://github.com/zclconf/go-cty/blob/65ead44d829d333450e12f04ce218a0099fa9bfd/cty/json/marshal.go#L11
// so thanks to the creators of this package
func marshal(val cty.Value, t cty.Type, path cty.Path, b *bytes.Buffer, isMapTupleObjectKey, isListSetItem bool) error {
	switch {
	case t.IsPrimitiveType():
		switch t {
		case cty.String:
			if !isMapTupleObjectKey {
				s, err := json.Marshal(val.AsString())
				if err != nil {
					return path.NewErrorf("not able to serialize value: %s", err)
				}
				b.Write(s)
			} else {
				b.WriteString(val.AsString())
			}
			if !isMapTupleObjectKey && !isListSetItem {
				b.WriteString("\n")
			}
			return nil
		case cty.Number:
			b.WriteString(val.AsBigFloat().Text('f', -1))
			b.WriteString("\n")
			return nil
		case cty.Bool:
			if val.True() {
				b.WriteString("true")
			} else {
				b.WriteString("false")
			}
			b.WriteString("\n")
			return nil
		default:
			return path.NewErrorf("value has unsupported primitive type: %s", t.FriendlyName())
		}
	case t.IsListType(), t.IsSetType():
		b.WriteRune('[')
		b.WriteString("\n")
		first := true
		ety := t.ElementType()
		it := val.ElementIterator()
		path := append(path, nil) // local override of 'path' with extra element
		for it.Next() {
			if !first {
				b.WriteString(",\n")
			}
			ek, ev := it.Element()
			path[len(path)-1] = cty.IndexStep{
				Key: ek,
			}
			err := marshal(ev, ety, path, b, false, true)
			if err != nil {
				return err
			}
			first = false
			b.WriteString("\n")
		}
		b.WriteRune(']')
		b.WriteString("\n")
		return nil
	case t.IsMapType():
		b.WriteRune('{')
		b.WriteString("\n")
		ety := t.ElementType()
		it := val.ElementIterator()
		path := append(path, nil) // local override of 'path' with extra element
		for it.Next() {
			ek, ev := it.Element()
			path[len(path)-1] = cty.IndexStep{
				Key: ek,
			}
			var err error
			err = marshal(ek, ek.Type(), path, b, true, false)
			if err != nil {
				return err
			}
			b.WriteRune('=')
			err = marshal(ev, ety, path, b, false, false)
			if err != nil {
				return err
			}
		}
		b.WriteRune('}')
		b.WriteString("\n")
		return nil
	case t.IsTupleType():
		b.WriteRune('[')
		b.WriteString("\n")
		etys := t.TupleElementTypes()
		it := val.ElementIterator()
		path := append(path, nil) // local override of 'path' with extra element
		i := 0
		for it.Next() {
			if i > 0 {
				b.WriteString(",\n")
			}
			ety := etys[i]
			ek, ev := it.Element()
			path[len(path)-1] = cty.IndexStep{
				Key: ek,
			}
			err := marshal(ev, ety, path, b, false, true)
			if err != nil {
				return err
			}
			i++
		}
		b.WriteString("\n")
		b.WriteRune(']')
		b.WriteString("\n")
		return nil
	case t.IsObjectType():
		b.WriteRune('{')
		b.WriteString("\n")
		atys := t.AttributeTypes()
		path := append(path, nil) // local override of 'path' with extra element

		names := make([]string, 0, len(atys))
		for k := range atys {
			names = append(names, k)
		}
		sort.Strings(names)

		for _, k := range names {
			aty := atys[k]
			av := val.GetAttr(k)
			path[len(path)-1] = cty.GetAttrStep{
				Name: k,
			}
			var err error
			err = marshal(cty.StringVal(k), cty.String, path, b, true, false)
			if err != nil {
				return err
			}
			b.WriteRune('=')
			err = marshal(av, aty, path, b, false, false)
			if err != nil {
				return err
			}
		}
		b.WriteRune('}')
		if !isListSetItem {
			b.WriteString("\n")
		}
		return nil
	case t.IsCapsuleType():
		rawVal := val.EncapsulatedValue()
		jsonVal, err := json.Marshal(rawVal)
		if err != nil {
			return path.NewError(err)
		}
		b.Write(jsonVal)
		return nil
	default:
		return path.NewErrorf("cannot convert to HCL %s", t.FriendlyName())
	}
}
