sudo cat > /etc/systemd/system/docker.service.d/http-proxy.conf <<-EOF
[Service]
Environment="http_proxy=http://192.168.14.1:1080/"
EOF

sudo systemctl daemon-reload
sudo systemctl restart docker