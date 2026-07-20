#!/bin/bash
#
set -e
#variables and config
INI_FILE="/etc/openuem-agent/openuem.ini"
# Network Settings
SERVER_IP_OR_DOMAIN="nats.openuem.delisys.local"
NATS_PORT="4433"
SFTP_PORT="2022"
VNC_PORT="1443"
CERT_DIR="/etc/openuem-agent/certificates"
TENANT_ID="2"  # Aus openuem-agent/org
SITE_ID="2"      # Aus openuem-agent/site

# Agent Certificate Data (Paste your block between the quotes)
AGENT_CERT_DATA="-----BEGIN CERTIFICATE-----
AGENT_CERT_DATA
-----END CERTIFICATE-----
"

# Agent Private Key Data
AGENT_KEY_DATA="-----BEGIN RSA PRIVATE KEY-----
AGENT_KEY_DATA
-----END RSA PRIVATE KEY-----
"

# CA Certificate Data
CA_CERT_DATA="-----BEGIN CERTIFICATE-----
CA_CERT_DATA
-----END CERTIFICATE-----
"

#SFTP Certificate Data
SFTP_CERT_DATA="-----BEGIN CERTIFICATE-----
SFTP_CERT_DATA
-----END CERTIFICATE-----
"

#generate a UUID if we don't have one:
if [ -f "$INI_FILE" ] && grep -q "UUID" "$INI_FILE"; then
    SYSTEM_UUID=$(grep "UUID" "$INI_FILE" | head -n 1 | awk -F'=' '{print $2}' | tr -d ' "')
else
    SYSTEM_UUID=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || uuidgen 2>/dev/null || echo "00000000-0000-0000-0000-000000000000")
fi


#
if ! command -v curl >/dev/null 2>&1; then
    echo "Error: curl is required but not installed, trying to install" >&2
    apt install -y curl
fi


sudo apt update -y

      # Configure OpenUEM repository
      if [ ! -f /usr/share/keyrings/openuem.gpg ]; then
        curl -fsSL https://apt.openuem.eu/pgp-key.public | gpg --dearmor -o /usr/share/keyrings/openuem.gpg >> "$LOG_FILE" 2>&1
      fi
      if [ ! -f /etc/apt/sources.list.d/openuem.list ]; then
        echo "deb [arch=amd64 signed-by=/usr/share/keyrings/openuem.gpg] https://apt.openuem.eu stable main" | tee /etc/apt/sources.list.d/openuem.list >> "$LOG_FILE" 2>&1
      fi

      apt-get update -y >> "$LOG_FILE" 2>&1
      apt-get install -y debconf crudini hwinfo systemd-container at net-tools x11vnc flatpak ansible python3-passlib zenity curl >> "$LOG_FILE" 2>&1

      # As silent install results in debconf waiting forever, we have to workaround this by patching the downloaded and unpacked package before install.

      mkdir -p /tmp/openuem-install
      chown -R _apt:root /tmp/openuem-install
      cd /tmp/openuem-install

      apt-get download openuem-agent >> "$LOG_FILE" 2>&1
      dpkg --unpack openuem-agent_*.deb >> "$LOG_FILE" 2>&1

      # Patching the "read" to always return 0
      sed -i "2i read() { for safe_var; do :; done; eval \"\\\$safe_var='1'\"; return 0; }" /var/lib/dpkg/info/openuem-agent.config /var/lib/dpkg/info/openuem-agent.postinst 2>/dev/null || true

      # Configure the installed patched package
      dpkg --configure openuem-agent >> "$LOG_FILE" 2>&1
      apt-get install -f -y >> "$LOG_FILE" 2>&1

      cd / && rm -rf /tmp/openuem-install

      # State retention and dynamic UUID generation
      INI_FILE="/etc/openuem-agent/openuem.ini"
      CERT_DIR="/etc/openuem-agent/certificates"

      # Replace placeholder with dynamic UUID if it has not been generated yet
      if [ -f "$INI_FILE" ] && grep -q "UUID_PLACEHOLDER" "$INI_FILE"; then
          SYSTEM_UUID=$(cat /proc/sys/kernel/random/uuid)
          sed -i "s/UUID_PLACEHOLDER/${SYSTEM_UUID}/g" "$INI_FILE"
          echo "Generated and set new UUID: ${SYSTEM_UUID}" >> "$LOG_FILE"
      fi

      # Fix permissions
      chmod 600 "$CERT_DIR/agent.key"
      chmod 644 $CERT_DIR/*.cer

      if id "openuem-agent" >/dev/null 2>&1; then
        chown -R openuem-agent:openUem-agent /etc/openuem-agent
      fi


# Install certificate files
echo "$AGENT_CERT_DATA" | sudo tee "$CERT_DIR/agent.cer" > /dev/null
echo "$AGENT_KEY_DATA" | sudo tee "$CERT_DIR/agent.key" > /dev/null
echo "$CA_CERT_DATA" | sudo tee "$CERT_DIR/ca.cer" > /dev/null
echo "$SFTP_CERT_DATA" | sudo tee "$CERT_DIR/sftp.cer" > /dev/null

# Clean out any old/commented entries to prevent duplicates
sudo sed -i '/NATSServers=/d' "$INI_FILE"
sudo sed -i '/SFTPPort=/d' "$INI_FILE"
sudo sed -i '/VNCProxyPort=/d' "$INI_FILE"
sudo sed -i '/AgentCert=/d' "$INI_FILE"
sudo sed -i '/AgentKey=/d' "$INI_FILE"
sudo sed -i '/CACert=/d' "$INI_FILE"

echo "create config file entries from variables"

sudo tee "$INI_FILE" > /dev/null << EOF
[Agent]
UUID=${SYSTEM_UUID}
Enabled=true
ExecuteTaskEveryXMinutes=5
Debug=false
DefaultFrequency=60
SFTPPort=${SFTP_PORT}
VNCProxyPort=${VNC_PORT}
TenantID=${TENANT_ID}
SiteID=${SITE_ID}

[NATS]
NATSServers=${SERVER_IP_OR_DOMAIN}:${NATS_PORT}

[Certificates]
CACert=${CERT_DIR}/ca.cer
AgentCert=${CERT_DIR}/agent.cer
AgentKey=${CERT_DIR}/agent.key
SFTPCert=${CERT_DIR}/sftp.cer
EOF

# compare paths from config with variables
sudo sed -i "s|^#\?AgentCert=.*|AgentCert=/etc/openuem-agent/certificates/agent.cer|" "$INI_FILE"
sudo sed -i "s|^#\?AgentKey=.*|AgentKey=/etc/openuem-agent/certificates/agent.key|" "$INI_FILE"
sudo sed -i "s|^#\?CACert=.*|CACert=/etc/openuem-agent/certificates/ca.cer|" "$INI_FILE"

# Enable the service
echo "Starting OpenUEM Agent..."
sudo systemctl daemon-reload
sudo systemctl enable --now openuem-agent

sudo chmod 600 /etc/openuem-agent/certificates/agent.key
sudo chmod 644 /etc/openuem-agent/certificates/*.cer
sudo chown -R openuem-agent:openuem-agent /etc/openuem-agent


echo "OpenUEM Agent successfully deployed!"
