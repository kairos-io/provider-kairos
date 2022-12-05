<h1 align="center">
  <br>
     <img width="184" alt="kairos-white-column 5bc2fe34" src="https://user-images.githubusercontent.com/2420543/193010398-72d4ba6e-7efe-4c2e-b7ba-d3a826a55b7d.png">
    <br>
<br>
</h1>

With Kairos you can build immutable, bootable Kubernetes and OS images for your edge devices as easily as writing a Dockerfile. Optional P2P mesh with distributed ledger automates node bootstrapping and coordination. Updating nodes is as easy as CI/CD: push a new image to your container registry and let secure, risk-free A/B atomic upgrades do the rest.

<h3 align="center">Kairos full-mesh support </h3>

This repository generates Kairos images with full-mesh support. full-mesh support currently is available only with k3s, and the provider follows strictly k3s releases.

To use Kairos with mesh support, either download the bootable medium in the releases, or either use kairos core with the provider-kairos bundles, during configuration like so:
```yaml
#node-config
install:
  bundles:
  - ....
```

## Upgrades

Upgrading can be done either via Kubernetes or manually with `kairos-agent upgrade --image <image>`, or you can list available versions with `kairos-agent upgrade list-releases`. 

Container images available for upgrades are pushed to quay, you can check out the [image matrix in our documentation](https://kairos.io/docs/reference/image_matrix/).
