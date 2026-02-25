package opa

import (
	"fmt"

	"github.com/kartverket/accesserator/pkg/utilities"
)

func renderDiscoveryNginxConf() string {
	return fmt.Sprintf(`server {
  listen %d;
  server_name _;
  # OPA bundle polling uses ETag/If-None-Match for efficient 304 responses.
  # Make this explicit instead of relying on nginx defaults.
  etag on;
  if_modified_since exact;

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
