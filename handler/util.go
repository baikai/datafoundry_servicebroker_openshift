package handler

import (
	//"errors"
	"encoding/base32"
	"encoding/base64"
	//"encoding/json"
	"time"
	//"fmt"
	"bytes"
	"crypto/tls"
	"net/http"
	
	//"github.com/pivotal-cf/brokerapi"
	//kapi "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/util/yaml"
)

var transport =  &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}

func request(timeout time.Duration, method, url, bearerToken string, body []byte) (*http.Response, error) {
	var req *http.Request
	var err error
	if len(body) == 0 {
		req, err = http.NewRequest(method, url, nil)
	} else {
		req, err = http.NewRequest(method, url, bytes.NewReader(body))
	}

	if err != nil {
		return nil, err
	}

	//for k, v := range headers {
	//	req.Header.Add(k, v)
	//}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	
	c := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
	return c.Do(req)
}

//===============================================================
//
//===============================================================

// maybe the replace order is important, so using slice other than map would be better
/*
func Yaml2Json(yamlTemplates []byte, replaces map[string]string) ([][]byte, error) {
	var err error

	for old, rep := range replaces {
		etcdTemplateData = bytes.Replace(etcdTemplateData, []byte(old), []byte(rep), -1)
	}

	templates := bytes.Split(etcdTemplateData, []byte("---"))
	for i := range templates {
		templates[i] = bytes.TrimSpace(templates[i])
		println("\ntemplates[", i, "] = ", string(templates[i]))
	}

	return templates, err
}
*/

/*
func Yaml2Json(yamlTemplates []byte, replaces map[string]string) ([][]byte, error) {
	var err error
	decoder := yaml.NewYAMLToJSONDecoder(bytes.NewBuffer(yamlData))
	_ = decoder


	for {
		var t interface{}
		err = decoder.Decode(&t)
		m, ok := v.(map[string]interface{})
		if ok {

		}
	}
}
*/

/*
func Yaml2Json(yamlTemplates []byte, replaces map[string]string) ([][]byte, error) {
	for old, rep := range replaces {
		yamlTemplates = bytes.Replace(yamlTemplates, []byte(old), []byte(rep), -1)
	}

	jsons := [][]byte{}
	templates := bytes.Split(yamlTemplates, []byte("---"))
	for i := range templates {
		//templates[i] = bytes.TrimSpace(templates[i])
		println("\ntemplates[", i, "] = ", string(templates[i]))

		json, err := yaml.YAMLToJSON(templates[i])
		if err != nil {
			return jsons, err
		}

		jsons = append(jsons, json)
		println("\njson[", i, "] = ", string(jsons[i]))
	}

	return jsons, nil
}
*/

type YamlDecoder struct {
	decoder *yaml.YAMLToJSONDecoder
	Err     error
}

func NewYamlDecoder(yamlData []byte) *YamlDecoder {
	return &YamlDecoder{
		decoder: yaml.NewYAMLToJSONDecoder(bytes.NewBuffer(yamlData)),
	}
}

func (d *YamlDecoder) Decode(into interface{}) *YamlDecoder {
	if d.Err == nil {
		d.Err = d.decoder.Decode(into)
	}

	return d
}

func NewElevenLengthID() string {
	t := time.Now().UnixNano()
	bs := make([]byte, 8)
	for i := uint(0); i < 8; i++ {
		bs[i] = byte((t >> i) & 0xff)
	}
	return string(base64.RawURLEncoding.EncodeToString(bs))
}

var base32Encoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567")

func NewThirteenLengthID() string {
	t := time.Now().UnixNano()
	bs := make([]byte, 8)
	for i := uint(0); i < 8; i++ {
		bs[i] = byte((t >> i) & 0xff)
	}

	dest := make([]byte, 16)
	base32Encoding.Encode(dest, bs)
	return string(dest[:13])
}

func NewTenLengthID() string {
	t := time.Now().UnixNano()
	t /= 100000 // unit: 0.1ms
	bs := make([]byte, 8)
	for i := uint(0); i < 8; i++ {
		bs[i] = byte((t >> i) & 0xff)
	}

	dest := make([]byte, 16)
	base32Encoding.Encode(dest, bs)
	return string(dest[:10])
}
