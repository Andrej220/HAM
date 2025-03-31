package dataservice 

import (
	"os"
	"encoding/json"
	"path/filepath"
)

type Options struct {
    Overwrite bool
    Prefix    string
    Indent    string
}

func WriteFile(data any, filename string, opts ...Options) error{

	opt := Options{
        Overwrite: true,
        Prefix:    "",
        Indent:    "    ",
    }

    if len(opts) > 0 {
        opt = opts[0]
    }

	if filename == "" {
        return os.ErrInvalid
    }

	if _, err := os.Stat(filename); !os.IsNotExist(err) && !opt.Overwrite {
        return os.ErrExist
    }

    err := os.MkdirAll(filepath.Dir(filename), 0755)
    if err != nil {
        return err
    }

    jsonBytes, err := json.MarshalIndent(data, opt.Prefix, opt.Indent)
    if err != nil {
        return err
    }
    return  os.WriteFile(filename, jsonBytes, 0644)
}

