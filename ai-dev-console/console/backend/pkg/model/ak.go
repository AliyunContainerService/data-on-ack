/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
*/
    
package model

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"k8s.io/klog"
	"time"
)

type AKInfo struct {
	AccessKeyId     string `json:"access.key.id"`
	AccessKeySecret string `json:"access.key.secret"`
	SecurityToken   string `json:"security.token"`
	Expiration      string `json:"expiration"`
	Keyring         string `json:"keyring"`
}

func (i *AKInfo) DecryptFromString(encodeTokenCfg string) error {
	err := json.Unmarshal([]byte(encodeTokenCfg), &i)
	if err != nil {
		return err
	}
	keyring := i.Keyring
	ak, err := Decrypt(i.AccessKeyId, []byte(keyring))
	if err != nil {
		return err
	}

	sk, err := Decrypt(i.AccessKeySecret, []byte(keyring))
	if err != nil {
		return err
	}

	token, err := Decrypt(i.SecurityToken, []byte(keyring))
	if err != nil {
		return err
	}
	layout := "2006-01-02T15:04:05Z"
	t, err := time.Parse(layout, i.Expiration)
	if err != nil {
		return err
	}
	if t.Before(time.Now()) {
		return errors.New("invalid token which is expired")
	}
	i.AccessKeyId = string(ak)
	i.AccessKeySecret = string(sk)
	i.SecurityToken = string(token)
	return nil
}

func PKCS5UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func Decrypt(s string, keyring []byte) ([]byte, error) {
	cdata, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		klog.Errorf("failed to decode base64 string, err: %v", err)
		return nil, err
	}
	block, err := aes.NewCipher(keyring)
	if err != nil {
		klog.Errorf("failed to new cipher, err: %v", err)
		return nil, err
	}
	blockSize := block.BlockSize()

	iv := cdata[:blockSize]
	blockMode := cipher.NewCBCDecrypter(block, iv)
	origData := make([]byte, len(cdata)-blockSize)

	blockMode.CryptBlocks(origData, cdata[blockSize:])

	origData = PKCS5UnPadding(origData)
	return origData, nil
}
