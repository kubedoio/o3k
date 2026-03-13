#!/bin/bash
set -e

# Disable default site
a2dissite 000-default || true

# Enable Horizon site
a2ensite horizon

# Start Apache in foreground
exec /usr/sbin/apache2ctl -DFOREGROUND
