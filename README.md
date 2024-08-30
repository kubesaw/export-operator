[![Validate Primer](https://github.com/cooktheryan/gitops-primer/actions/workflows/validate-primer.yaml/badge.svg)](https://github.com/cooktheryan/gitops-primer/actions/workflows/validate-primer.yaml)

# export-operator
Export Operator is an operator based on GitOps Primer (https://github.com/migtools/gitops-primer) that can be deployed 
within a Kubernetes environment to export objects out of the cluster and either make them available for download or
store them within a Git repository.

## Developing
If you would like to test or develop using the Export Operator deploy Minikube(https://minikube.sigs.k8s.io/docs/start/) or 
Kind(https://kind.sigs.k8s.io/) and run the following.

```
make install
make run
```

## Deploying without OLM
If you would like to run the Export Operator within your environment. 
```
make deploy
```

## Deploying with OLM
If you would like to run the Export Operator within your environment that has OLM
```
oc create ns export-operator-system
operator-sdk run bundle quay.io/kubesaw/export-operator-bundle:v0.0.1 --namespace export-operator-system
```

## Running
Although there are two examples that are given within the examples directory the only one that will be usable to you is 
*export-to-git.yaml*. This is because *download-export.yaml* requires a different pod to present the downloadable 
content to you. The information below will focus on the git method.

WARNING: Please use a private git repository in this example!

A secret containing an SSH key that is linked to the Git Repository must be created before running GitOps Primer. Follow 
the steps to add a new SSH key to your GitHub account 
(https://docs.github.com/en/github/authenticating-to-github/connecting-to-github-with-ssh/adding-a-new-ssh-key-to-your-github-account).

```
oc create secret generic secret-key --from-file=id_rsa=~/.ssh/id_rsa
```

Now that the SSH key is loaded modify the file examples/export-to-git.yaml to define the git branch and private 
repository to use and then deploy.

```
oc create -f examples/export-to-git.yaml
```

After the job completes, items will exist within your git repository.

