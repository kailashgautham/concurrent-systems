#!/bin/bash

# Navigate to the directory containing the tests folder
# cd /path/to/your/tests/folder/parent

# Loop through each .in file in the tests folder
for file in tests/*.in; do
    ./grader engine < "$file"
    echo "--------------------------------"
done
