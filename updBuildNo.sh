#!/bin/bash

BUILD_NO=$(git rev-list --count HEAD)

echo "BuildNo: $BUILD_NO" >.build_no

for i in $* ; do

	ed $i <<XXxx
/BuildNo =/s/= "[0-9][0-9][0-9]*"/= "0$BUILD_NO"/
w
q
XXxx

done

