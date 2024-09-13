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

package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func ReadSystemdLine(fileName string) (string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	result := ""
	for {
		tempByte, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if strings.Contains(string(tempByte), "name=systemd") {
			result = string(tempByte)
			break
		}
	}
	if result == "" {
		return "", fmt.Errorf("can't get container id")
	}
	return result, nil
}

func GetContainerID(cgroupContent string) string {
	tempStr := cgroupContent
	tempArr := strings.Split(tempStr, "/")

	tempStr = tempArr[len(tempArr)-1]
	tempArr = strings.Split(tempStr, "-")

	tempStr = tempArr[len(tempArr)-1]
	tempArr = strings.Split(tempStr, ".")

	return tempArr[0]
}
