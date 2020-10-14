# Examples

Each of those examples assume that you will deploy them to a
[_local cluster_](https://github.com/GoogleContainerTools/skaffold/tree/master/examples#examples)
such as `minikube` or `Docker Desktop`.  For example:

```
cd getting-started
skaffold dev
```

When deploying to a _remote cluster_ such as [GKE](https://cloud.google.com/kubernetes-engine),
you have to point Skaffold to your default image repository in one of four ways:

* flag: `skaffold dev --default-repo <myrepo>`
* env var: `SKAFFOLD_DEFAULT_REPO=<myrepo> skaffold dev`
* global skaffold config (one time): `skaffold config set --global default-repo <myrepo>`
* skaffold config for current kubectl context: `skaffold config set default-repo <myrepo>`

Read the [Quickstart](https://skaffold.dev/docs/quickstart/) for more detailed instructions.

----

These examples are made to work with the latest release of Skaffold.

If you are running Skaffold at HEAD or have built it from source, please use the examples at `integration/examples`.

*Note for contributors*: If you wish to make changes to these examples, please edit the ones at `integration/examples`,
as those will be synced on release.
