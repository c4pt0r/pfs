#!/bin/bash

# Example usage of agfs-shell2

echo "=== Basic Commands ==="
echo "Testing echo:"
uv run agfs-shell2 "echo Hello, World!"

echo -e "\n=== Testing cat with stdin ==="
echo "hello world" | uv run agfs-shell2 cat

echo -e "\n=== Simple Pipeline ==="
echo "Testing echo | grep:"
echo "hello world" | uv run agfs-shell2 "cat | grep hello"

echo -e "\n=== Three-stage Pipeline ==="
echo "Testing cat | grep | wc:"
echo "hello world" | uv run agfs-shell2 "cat | grep hello | wc -c"

echo -e "\n=== Line Counting ==="
printf "line1\nline2\nline3\n" | uv run agfs-shell2 "cat | wc -l"

echo -e "\n=== Character Translation ==="
uv run agfs-shell2 "echo hello | tr h H"

echo -e "\n=== Sort and Uniq ==="
printf "apple\nbanana\napple\ncherry\nbanana\n" | uv run agfs-shell2 "cat | sort | uniq"

echo -e "\n=== Head and Tail ==="
printf "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n" | uv run agfs-shell2 "cat | head -n 3"
printf "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n" | uv run agfs-shell2 "cat | tail -n 3"

echo -e "\n=== Complex Pipeline ==="
printf "apple pie\nbanana split\napple juice\ncherry pie\n" | \
  uv run agfs-shell2 "cat | grep pie | sort | wc -l"

echo -e "\nAll tests completed!"
