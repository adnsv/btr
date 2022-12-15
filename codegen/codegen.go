package codegen

import (
	"bytes"
	"fmt"
	"io/ioutil"
)

type Config struct {
	TopMatter       Matter                       `json:"top-matter,omitempty" yaml:"top-matter,omitempty"`
	BottomMatter    Matter                       `json:"bottom-matter,omitempty" yaml:"bottom-matter,omitempty"`
	CPP             *CPPConfig                   `json:"c++,omitempty" yaml:"c++,omitempty"`
	OnBeforeWrite   func(path string)            `json:"-" yaml:"-"`
	OnWriteSucceded func(path string)            `json:"-" yaml:"-"`
	OnWriteFailed   func(path string, err error) `json:"-" yaml:"-"`
}

type Matter map[string][]string

func (m *Matter) Lines(langs ...string) []string {
	ret := []string{}
	for _, lang := range langs {
		ret = append(ret, (*m)[lang]...)
	}
	return ret
}

type CPPConfig struct {
}

type FileWriter interface {
	FilePath() string
	WriteOut() error
}

type Buffer struct {
	FileWriter
	bytes.Buffer
	config   *Config
	filePath string
}

func NewBuffer(filepath string, config *Config) *Buffer {
	return &Buffer{filePath: filepath, config: config}
}

func (b *Buffer) FilePath() string {
	return b.filePath
}

func (b *Buffer) WriteOut() error {
	if b.config != nil && b.config.OnBeforeWrite != nil {
		b.config.OnBeforeWrite(b.filePath)
	}
	err := ioutil.WriteFile(b.filePath, b.Bytes(), 0644)
	if b.config != nil {
		if err == nil {
			if b.config.OnWriteSucceded != nil {
				b.config.OnWriteSucceded(b.filePath)
			}
		} else {
			if b.config.OnWriteFailed != nil {
				b.config.OnWriteFailed(b.filePath, err)
			}
		}

	}
	return err
}

func (b *Buffer) WriteBottomMatter() {
	// noop
}

func (b *Buffer) Print(v ...interface{}) {
	b.WriteString(fmt.Sprint(v...))
}

func (b *Buffer) Printf(format string, v ...interface{}) {
	b.WriteString(fmt.Sprintf(format, v...))
}

func (b *Buffer) WriteLines(lines ...string) {
	for _, line := range lines {
		fmt.Fprint(b, line, "\n")
	}
}

func (b *Buffer) BeginCppNamespace(ns string) {
	if len(ns) > 0 {
		fmt.Fprintf(b, "namespace %s {\n\n", ns)
	}
}

func (b *Buffer) EndCppNamespace(ns string) {
	if len(ns) > 0 {
		fmt.Fprint(b, "\n}\n")
	}
}
