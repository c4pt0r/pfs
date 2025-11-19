#!/bin/bash

echo "=== Testing Redirection Features ==="

# Clean up
rm -f /tmp/agfs_test_*.txt

echo "=== 1. Output Redirection (>) ==="
uv run agfs-shell2 "echo 'hello world' > /tmp/agfs_test_out.txt"
echo "Expected: hello world"
echo "Got:      $(cat /tmp/agfs_test_out.txt)"

echo -e "\n=== 2. Append Redirection (>>) ==="
uv run agfs-shell2 "echo 'line 1' > /tmp/agfs_test_append.txt"
uv run agfs-shell2 "echo 'line 2' >> /tmp/agfs_test_append.txt"
uv run agfs-shell2 "echo 'line 3' >> /tmp/agfs_test_append.txt"
echo "Expected: 3 lines"
echo "Got:"
cat /tmp/agfs_test_append.txt

echo -e "\n=== 3. Input Redirection (<) ==="
echo "test data from file" > /tmp/agfs_test_in.txt
echo "Expected: test data from file"
echo "Got:      $(uv run agfs-shell2 'cat < /tmp/agfs_test_in.txt')"

echo -e "\n=== 4. Pipeline with Output Redirection ==="
uv run agfs-shell2 "echo 'apple banana cherry' | tr ' ' '\n' > /tmp/agfs_test_pipe.txt"
echo "Expected: 3 lines (apple, banana, cherry)"
echo "Got:"
cat /tmp/agfs_test_pipe.txt

echo -e "\n=== 5. Input Redirection with Pipeline ==="
printf "line1\nline2\nline3\n" > /tmp/agfs_test_pipe_in.txt
echo "Expected: 3"
echo "Got:      $(uv run agfs-shell2 'wc -l < /tmp/agfs_test_pipe_in.txt')"

echo -e "\n=== 6. Complex: Input + Pipeline + Output ==="
printf "apple\nbanana\napple\ncherry\nbanana\n" > /tmp/agfs_test_complex.txt
uv run agfs-shell2 "cat < /tmp/agfs_test_complex.txt | sort | uniq > /tmp/agfs_test_result.txt"
echo "Expected: apple, banana, cherry (sorted, unique)"
echo "Got:"
cat /tmp/agfs_test_result.txt

echo -e "\n=== 7. Overwrite vs Append ==="
uv run agfs-shell2 "echo 'first' > /tmp/agfs_test_overwrite.txt"
uv run agfs-shell2 "echo 'second' > /tmp/agfs_test_overwrite.txt"
echo "Expected (overwrite): second"
echo "Got:                  $(cat /tmp/agfs_test_overwrite.txt)"

uv run agfs-shell2 "echo 'first' > /tmp/agfs_test_no_overwrite.txt"
uv run agfs-shell2 "echo 'second' >> /tmp/agfs_test_no_overwrite.txt"
echo "Expected (append): first, second"
echo "Got:"
cat /tmp/agfs_test_no_overwrite.txt

echo -e "\n=== 8. Multiple Pipes with Redirections ==="
printf "10\n5\n20\n15\n" > /tmp/agfs_test_numbers.txt
uv run agfs-shell2 "cat < /tmp/agfs_test_numbers.txt | sort | head -n 2 > /tmp/agfs_test_top2.txt"
echo "Expected: 10, 15 (sorted, first 2)"
echo "Got:"
cat /tmp/agfs_test_top2.txt

echo -e "\n=== Cleanup ==="
rm -f /tmp/agfs_test_*.txt
echo "All redirection tests completed!"
