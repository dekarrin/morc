#!/bin/bash

# this file builds distributions. By default, for 3 major operating systems:
# Mac (darwin), Windows, and Linux

# fail immediately on first error
set -eo pipefail

# assumes this script is in the repo root:
cd "$(dirname "$0")/"

if [ -z "$PLATFORMS" ]
then
  PLATFORMS="
darwin/amd64
windows/amd64
linux/amd64"
fi


# only do skip tests if tests have already been done.
[ "$1" = "--skip-tests" ] && skip_tests=1

BINARY_NAME="morc"
MAIN_PACKAGE_PATH="./cmd/morc"
ARCHIVE_NAME="morc"

tar_cmd=tar
if [ "$(uname -s)" = "Darwin" ]
then
	if tar --version | grep bsdtar >/dev/null 2>&1
	then
		if ! gtar --version >/dev/null 2>&1
		then
			echo "You appear to be running on a mac where 'tar' is BSD tar." >&2
			echo "This will cause issues due to its adding of non-standard headers." >&2
			echo "" >&2
			echo "Please install GNU tar and make it available as 'gtar' with:" >&2
			echo "  brew install gnu-tar" >&2
			echo "And then try again" >&2
			exit 1
		else
			tar_cmd=gtar
		fi
	fi
fi


version="$(go run $MAIN_PACKAGE_PATH --version | awk '{print $NF;}')"
if [ -z "$version" ]
then
	echo "could not get version number; abort" >&2
	exit 1
fi

echo "Creating distributions for $ARCHIVE_NAME version $version"

rm -rf "$BINARY_NAME" "$BINARY_NAME.exe"
rm -rf "source.tar.gz"
rm -rf *-source/

if [ -z "$skip_tests" ]
then
  go clean
  go get ./... || { echo "could not install dependencies; abort" >&2 ; exit 1 ; }
  echo "Running unit tests..."
  if go test -count 1 -timeout 30s ./...
  then
    echo "Unit tests passed"
  else
    echo "Unit tests failed; fix the tests and then try again" >&2
    exit 1
  fi
else
  echo "Skipping tests due to --skip-tests flag; make sure they are executed elsewhere"
fi

source_dir="$ARCHIVE_NAME-$version-source"
git archive --format=tar --prefix="$source_dir/" HEAD | "$tar_cmd" xf -
"$tar_cmd" czf "source.tar.gz" "$source_dir"
rm -rf "$source_dir"

for p in $PLATFORMS
do
  current_os="${p%/*}"
  current_arch="${p#*/}"
  echo "Building for $current_os on $current_arch..."

  dist_bin_name="$BINARY_NAME"
  if [ "$current_os" = "windows" ]
  then
    dist_bin_name="${BINARY_NAME}.exe"
  fi

  go clean
  env CGO_ENABLED=0 GOOS="$current_os" GOARCH="$current_arch" go build -o "$dist_bin_name" "$MAIN_PACKAGE_PATH" || { echo "build failed; abort" >&2 ; exit 1 ; }

  dist_versioned_name="$ARCHIVE_NAME-$version-$current_os-$current_arch"
  dist_latest_name="$ARCHIVE_NAME-latest-$current_os-$current_arch"

  distfolder="$dist_versioned_name"
  rm -rf "$distfolder" "$dist_latest_name.tar.gz" "$dist_versioned_name.tar.gz"
  mkdir "$distfolder"
  cp README.md RELEASES.md LICENSE source.tar.gz "$distfolder"

  if [ "$current_os" != "windows" ]
  then
    # no need to set executable bit on windows
    chmod +x "$dist_bin_name"
  fi
  mv $dist_bin_name "$distfolder/"
  $tar_cmd czf "$dist_versioned_name.tar.gz" "$distfolder"
  rm -rf "$distfolder"

  echo "$dist_versioned_name.tar.gz"
  cp "$dist_versioned_name.tar.gz" "$dist_latest_name.tar.gz"
  echo "$dist_latest_name.tar.gz"
done

rm -rf source.tar.gz
