package util

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

func ParseAddr(addr string) (string, int32, error) {
	arr := strings.Split(addr, ":")
	if len(arr) != 2 || len(arr[0]) == 0 || len(arr[1]) == 0 {
		return "", 0, errors.New("invalid address")
	}
	port, err := strconv.Atoi(arr[1])
	if err != nil {
		return "", 0, err
	}
	return arr[0], int32(port), nil
}

func ScanAddress(defaultIP string) string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	ip := scanner.Text()
	if len(strings.TrimSpace(ip)) == 0 {
		return defaultIP
	}
	return ip
}
