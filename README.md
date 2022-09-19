<h1 align="center">
  <br>
     <img src="https://user-images.githubusercontent.com/2420543/153508410-a806a385-ae3e-417e-b87e-7472f21689e3.png" width=500>
	<br>
<br>
</h1>

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

Container images available for upgrades are pushed to quay:

- [OpenSUSE based](https://quay.io/repository/kairos/kairos-opensuse)
- [Alpine based](https://quay.io/repository/kairos/kairos-alpine)
- [OpenSUSE RaspberryPi 3/4](https://quay.io/repository/kairos/kairos-opensuse-arm-rpi)
- [Alpine RaspberryPi 3/4](https://quay.io/repository/kairos/kairos-alpine-arm-rpi)