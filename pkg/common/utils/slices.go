// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Intel Corporation

package utils

func Filter[T any](slice []T, filter func(T) bool) []T {
	var n []T
	for _, e := range slice {
		if filter(e) {
			n = append(n, e)
		}
	}
	return n
}
