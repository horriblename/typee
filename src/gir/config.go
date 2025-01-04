package gir

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/linuxdeepin/go-gir/generator/gi"
)

type Config struct {
	Blacklist       map[string]map[string]bool
	Whitelist       map[string]map[string]bool
	MethodBlacklist map[string]map[string]bool
	MethodWhitelist map[string]map[string]bool
	ExtraTypes      map[string]string
}

func ParseConfig(in io.Reader) (Config, error) {
	type tmp struct {
		Blacklist       map[string][]string `json:"blacklist"`
		Whitelist       map[string][]string `json:"whitelist"`
		MethodBlacklist map[string][]string `json:"method-blacklist"`
		MethodWhitelist map[string][]string `json:"method-whitelist"`
		ExtraTypes      map[string]string   `json:"extra-types"`
	}

	t := tmp{}
	decoder := json.NewDecoder(in)
	if err := decoder.Decode(&t); err != nil {
		return Config{}, err
	}

	return Config{
		Blacklist:       mapMap(t.Blacklist, sliceToSet),
		Whitelist:       mapMap(t.Whitelist, sliceToSet),
		MethodBlacklist: mapMap(t.MethodBlacklist, sliceToSet),
		MethodWhitelist: mapMap(t.MethodWhitelist, sliceToSet),
		ExtraTypes:      t.ExtraTypes,
	}, nil
}

func (this *Config) is_object_blacklisted(bi *gi.BaseInfo) bool {
	switch bi.Type() {
	case gi.INFO_TYPE_UNION:
		return this.is_blacklisted("unions", bi.Name())
	case gi.INFO_TYPE_STRUCT:
		return this.is_blacklisted("structs", bi.Name())
	case gi.INFO_TYPE_ENUM, gi.INFO_TYPE_FLAGS:
		return this.is_blacklisted("enums", bi.Name())
	case gi.INFO_TYPE_CONSTANT:
		return this.is_blacklisted("constants", bi.Name())
	case gi.INFO_TYPE_CALLBACK:
		return this.is_blacklisted("callbacks", bi.Name())
	case gi.INFO_TYPE_FUNCTION:
		c := bi.Container()
		if c != nil {
			return this.is_method_blacklisted(c.Name(), bi.Name())
		}
		return this.is_blacklisted("functions", bi.Name())
	case gi.INFO_TYPE_INTERFACE:
		return this.is_blacklisted("interfaces", bi.Name())
	case gi.INFO_TYPE_OBJECT:
		return this.is_blacklisted("objects", bi.Name())
	default:
		fmt.Printf("TODO: %s (%s)\n", bi.Name(), bi.Type())
		return true
	}
}

func (this *Config) is_blacklisted(section, entry string) bool {
	// check if the entry is in the blacklist
	if section_map, ok := this.Blacklist[section]; ok {
		if _, ok := section_map[entry]; ok {
			return true
		}
	}

	// check if the entry is missing from the whitelist
	if section_map, ok := this.Whitelist[section]; ok {
		if _, ok := section_map[entry]; !ok {
			return true
		}
	}

	return false
}

func (this *Config) is_method_blacklisted(class, method string) bool {
	// don't want to see these
	if method == "ref" || method == "unref" {
		return true
	}

	if class_map, ok := this.MethodBlacklist[class]; ok {
		if _, ok := class_map[method]; ok {
			return true
		}
	}

	if class_map, ok := this.MethodWhitelist[class]; ok {
		if _, ok := class_map[method]; !ok {
			return true
		}
	}

	return false
}

func (this *Config) writeExtraTypes(w io.Writer) error {
	type decl struct {
		name string
		typ  string
	}

	extras := make([]decl, 0, len(this.ExtraTypes))
	for k, v := range this.ExtraTypes {
		extras = append(extras, decl{
			name: k,
			typ:  v,
		})
	}

	slices.SortFunc(extras, func(a, b decl) int {
		if a.name == b.name {
			return 0
		} else if a.name < b.name {
			return -1
		}
		return 1
	})

	for _, d := range extras {
		if _, err := fmt.Fprintf(w, "(type %s %s)\n", d.name, d.typ); err != nil {
			return err
		}
	}

	return nil
}
