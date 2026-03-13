#!/bin/bash
set -e

# Remove default Apache site
rm -f /etc/apache2/sites-enabled/000-default.conf

# Start Apache
exec /usr/sbin/apache2ctl -DFOREGROUND
