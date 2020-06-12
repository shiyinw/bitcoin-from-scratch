#!/bin/bash

# This script starts a series of tests.

echo "Killall go before the tests!"
killall go

echo "Compile"
./compile.sh

# Initialize the ports
lsof -n -i4TCP:50051 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i4TCP:50052 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i4TCP:50053 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i4TCP:50054 | grep LISTEN | awk '{ print $2 }' | xargs kill
lsof -n -i4TCP:50055 | grep LISTEN | awk '{ print $2 }' | xargs kill

echo "Tests start..."

I=0
for filename in test/test*.go; do
	echo "Running " $filename
	TESTS[$I]=$filename
	go run $filename
	RESULT[$I]=$?
	I=$[I + 1]
done

# Define colors for summary.
BGre='\e[1;32m';
BRed='\e[1;31m';
NoColor='\033[0m';

echo "==== Test Summary ===="
TOTAL=$I
SUM=0
N=$[TOTAL-1]
for I in `seq 0 $N`; do
	filename=${TESTS[$I]}
	code=${RESULT[$I]}
	printf "$filename :"
	if [ "$code" -eq "0" ]; then
		printf "${BGre} PASS ${NoColor} \n"
		SUM=$[SUM + 1]
	else
		printf "${BRed} FAIL ${NoColor} \n"
	fi
done
echo "Overall: $SUM / $TOTAL "
