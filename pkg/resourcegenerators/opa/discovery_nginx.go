package opa

import (
	"fmt"

	"github.com/kartverket/accesserator/pkg/utilities"
)

func renderDiscoveryNginxConf() string {
	return fmt.Sprintf(`server {
  listen %d;
  server_name _;

  location = %s {
    alias %s/%s;
    default_type application/gzip;
    add_header Cache-Control "no-store";
  }

  location = %s {
    alias %s;
    default_type application/gzip;
    add_header Cache-Control "no-store";
  }
}
`,
		opaDiscoveryContainerPort,
		opaDiscoveryPath,
		opaDiscoveryNginxConfigMountPath,
		utilities.OpaDiscoveryBundleFileName,
		GetOpaDiscoveryBundleResourcePath(),
		getOpaDiscoveryMirroredBundleFilePath(),
	)
}
