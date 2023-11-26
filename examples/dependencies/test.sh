got="$(cat $TEST_TMPDIR/out.txt)"

if [ "$got" != "hello world" ]; then
  echo "Received: '$got'"
  exit 1
fi;
