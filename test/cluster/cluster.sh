#!/bin/bash

KIND_EXECUTABLE="kind"  # Replace with the actual path to the KIND executable

# Define the cluster_name variable with a default value of "test cluster one"
cluster_name=${cluster_name:-"test-cluster-1"}

# Define the config_file variable with a default value of "kind-config.yaml"
config_file=${config_file:-"kind-config.yaml"}

# Function to call the KIND executable with the nodes variable as an argument
create_cluster() {
    $KIND_EXECUTABLE create cluster --name $cluster_name --config $config_file
}

# Call the function
create_cluster
