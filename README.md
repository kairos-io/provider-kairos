<h1 align="center">
  <br>
     <img src="https://user-images.githubusercontent.com/2420543/153508410-a806a385-ae3e-417e-b87e-7472f21689e3.png" width=500>
	<br>
<br>
</h1>

<h3 align="center">c3OS full-mesh support </h3>

This repository generates c3OS images with full-mesh support. full-mesh support currently is available only with k3s, and the provider follows strictly k3s releases.

To use c3os with mesh support, either download the bootable medium in the releases, or either use c3os light/core with the provider-c3os bundles, during configuratio
 like so:
```yaml
#node-config

bundles:
- ....
```