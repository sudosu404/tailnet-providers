package notif

import (
	"bytes"
	"strings"

	"github.com/bytedance/sonic"
	gperr "github.com/yusing/goutils/errs"
)

type (
	LogField struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	LogFormat string
	LogBody   interface {
		Format(format LogFormat) ([]byte, error)
	}
)

type (
	FieldsBody       []LogField
	ListBody         []string
	MessageBody      string
	MessageBodyBytes []byte
	errorBody        struct {
		Error error
	}
)

const (
	LogFormatMarkdown LogFormat = "markdown"
	LogFormatPlain    LogFormat = "plain"
	LogFormatRawJSON  LogFormat = "json" // internal use only
)

func MakeLogFields(fields ...LogField) LogBody {
	return FieldsBody(fields)
}

func ErrorBody(err error) LogBody {
	return errorBody{Error: err}
}

func (f *FieldsBody) Add(name, value string) {
	*f = append(*f, LogField{Name: name, Value: value})
}

func (f FieldsBody) Format(format LogFormat) ([]byte, error) {
	switch format {
	case LogFormatMarkdown:
		var msg bytes.Buffer
		for _, field := range f {
			msg.WriteString("#### ")
			msg.WriteString(field.Name)
			msg.WriteByte('\n')
			msg.WriteString(field.Value)
			msg.WriteByte('\n')
		}
		return msg.Bytes(), nil
	case LogFormatPlain:
		var msg bytes.Buffer
		for _, field := range f {
			msg.WriteString(field.Name)
			msg.WriteString(": ")
			msg.WriteString(field.Value)
			msg.WriteByte('\n')
		}
		return msg.Bytes(), nil
	case LogFormatRawJSON:
		return sonic.Marshal(f)
	}
	return f.Format(LogFormatMarkdown)
}

func (l ListBody) Format(format LogFormat) ([]byte, error) {
	switch format {
	case LogFormatPlain:
		return []byte(strings.Join(l, "\n")), nil
	case LogFormatMarkdown:
		var msg bytes.Buffer
		for _, item := range l {
			msg.WriteString("* ")
			msg.WriteString(item)
			msg.WriteByte('\n')
		}
		return msg.Bytes(), nil
	case LogFormatRawJSON:
		return sonic.Marshal(l)
	}
	return l.Format(LogFormatMarkdown)
}

func (m MessageBody) Format(format LogFormat) ([]byte, error) {
	switch format {
	case LogFormatPlain, LogFormatMarkdown:
		return []byte(m), nil
	case LogFormatRawJSON:
		return sonic.Marshal(m)
	}
	return []byte(m), nil
}

func (m MessageBodyBytes) Format(format LogFormat) ([]byte, error) {
	switch format {
	case LogFormatRawJSON:
		return sonic.Marshal(string(m))
	}
	return m, nil
}

func (e errorBody) Format(format LogFormat) ([]byte, error) {
	switch format {
	case LogFormatRawJSON:
		return sonic.Marshal(e.Error)
	case LogFormatPlain:
		return gperr.Plain(e.Error), nil
	case LogFormatMarkdown:
		return gperr.Markdown(e.Error), nil
	}
	return gperr.Markdown(e.Error), nil
}
