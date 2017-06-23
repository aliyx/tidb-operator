package k8sutil

var tidbNamespaceYaml = `
kind: Namespace
apiVersion: v1
metadata:
  name: {{namespace}}
`
