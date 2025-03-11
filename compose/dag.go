/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package compose

import (
	"context"
	"fmt"
)

func dagChannelBuilder(dependencies []string) channel {
	waitList := make(map[string]dagChannelState, len(dependencies))
	for _, dep := range dependencies {
		waitList[dep] = unready
	}
	return &dagChannel{
		values:   make(map[string]any),
		waitList: waitList,
	}
}

type dagChannelState int

const (
	unready dagChannelState = iota
	success
	skipped
)

type dagChannel struct {
	values   map[string]any
	waitList map[string]dagChannelState
	value    any
	skipped  bool
}

func (ch *dagChannel) update(ctx context.Context, ins map[string]any) error {
	if ch.skipped {
		return nil
	}

	for k, v := range ins {
		if _, ok := ch.values[k]; ok {
			return fmt.Errorf("dag channel update, calculate node repeatedly: %s", k)
		}
		ch.values[k] = v
		if _, ok := ch.waitList[k]; ok {
			ch.waitList[k] = success
		}
	}

	return ch.tryUpdateValue()
}

func (ch *dagChannel) get(ctx context.Context) (any, error) {
	if ch.skipped {
		return nil, fmt.Errorf("dag channel has been skipped")
	}
	if ch.value == nil {
		return nil, fmt.Errorf("dag channel not ready, value is nil")
	}
	v := ch.value
	ch.value = nil
	return v, nil
}

func (ch *dagChannel) ready(ctx context.Context) bool {
	if ch.skipped {
		return false
	}
	return ch.value != nil
}

func (ch *dagChannel) reportSkip(keys []string) (bool, error) {
	for _, k := range keys {
		if _, ok := ch.waitList[k]; ok {
			ch.waitList[k] = skipped
		}
	}

	allSkipped := true
	for _, state := range ch.waitList {
		if state != skipped {
			allSkipped = false
			break
		}
	}
	ch.skipped = allSkipped

	var err error
	if !allSkipped {
		err = ch.tryUpdateValue()
	}

	return allSkipped, err
}

func (ch *dagChannel) reportSuccessWithoutValue(key string) error {
	if _, ok := ch.waitList[key]; ok {
		ch.waitList[key] = success
	}

	return ch.tryUpdateValue()
}

func (ch *dagChannel) tryUpdateValue() error {
	for _, state := range ch.waitList {
		if state == unready {
			return nil
		}
	}

	values := mapToList(ch.values)
	if len(values) == 1 {
		ch.value = values[0]
		return nil
	}
	v, err := mergeValues(values)
	if err != nil {
		return err
	}
	ch.value = v
	return nil

}
