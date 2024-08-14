/*
Copyright 2022 Zheng Dayu
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"time"

	"github.com/go-ole/go-ole"
)

func ToIDispatchErr(result *ole.VARIANT, err error) (*ole.IDispatch, error) {
	if err != nil {
		return nil, err
	}
	return variantToIDispatch(result), nil
}

func variantToIDispatch(v *ole.VARIANT) *ole.IDispatch {
	value := v.Value()
	if value == nil {
		return nil
	}
	return v.ToIDispatch()
}

func ToTimeErr(result *ole.VARIANT, err error) (*time.Time, error) {
	if err != nil {
		return nil, err
	}
	return variantToTime(result), nil
}

func variantToTime(v *ole.VARIANT) *time.Time {
	value := v.Value()
	if value == nil {
		return nil
	}
	valueTime := value.(time.Time)
	return &valueTime
}

func ToInt32Err(result *ole.VARIANT, err error) (int32, error) {
	if err != nil {
		return 0, err
	}
	return variantToInt32(result), nil
}

func variantToInt32(v *ole.VARIANT) int32 {
	value := v.Value()
	if value == nil {
		return 0
	}
	return value.(int32)
}
