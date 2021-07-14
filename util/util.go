package util

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// StringInSlice checks whether the string is in the slice
func StringInSlice(target string, list []string) bool {
	for _, ele := range list {
		if target == ele {
			return true
		}
	}
	return false
}

func VerifyStatusCode(response *http.Response, expected int) error {
	if response.StatusCode != expected {
		message, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("code = %v, err = %s", response.StatusCode, message)
	}
	return nil
}

// HandleErrs checks a list of errors, return the first error encountered,
// nil otherwise.
func HandleErrs(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}