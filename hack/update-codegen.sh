#!/bin/bash

# The only argument this script should ever be called with is '--verify-only'
set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(CDPATH='' cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

"${CODEGEN_PKG}/generate-groups.sh" "all" \
  github.com/simonswine/rocklet/pkg/client github.com/simonswine/rocklet/pkg/apis\
  vacuum:v1alpha1
