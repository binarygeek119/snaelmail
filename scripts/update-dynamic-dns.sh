#!/bin/bash
# Dynamic DNS Update Script for Ephemeral IP
# Automatically updates DNS when VM IP changes

set -e

# Configuration
DOMAIN="snailmail.org"
SUBDOMAINS=("www" "mail" "imap")
CHECK_INTERVAL=300  # Check every 5 minutes
LOG_FILE="/opt/snailmail/logs/ip-update.log"

# DuckDNS Configuration (FREE dynamic DNS service)
# Sign up at https://www.duckdns.org/
DUCKDNS_DOMAIN="snailmail"  # Your DuckDNS subdomain
DUCKDNS_TOKEN="YOUR_DUCKDNS_TOKEN_HERE"  # Get from DuckDNS dashboard

# ============================================================================
# FUNCTIONS
# ============================================================================

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

get_current_ip() {
    # Get current VM IP from GCP metadata
    curl -s -H "Metadata-Flavor: Google" \
        http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip
}

get_last_ip() {
    # Get last known IP from file
    if [ -f "/opt/snailmail/last-ip.txt" ]; then
        cat /opt/snailmail/last-ip.txt
    else
        echo ""
    fi
}

save_ip() {
    echo "$1" > /opt/snailmail/last-ip.txt
}

update_duckdns() {
    local ip=$1
    local response
    
    response=$(curl -s "https://www.duckdns.org/update?domains=$DUCKDNS_DOMAIN&token=$DUCKDNS_TOKEN&ip=$ip")
    
    if [ "$response" = "OK" ]; then
        log "✓ DuckDNS updated: $DUCKDNS_DOMAIN.duckdns.org → $ip"
        return 0
    else
        log "✗ DuckDNS update failed: $response"
        return 1
    fi
}

update_gcp_dns() {
    local ip=$1
    
    # Update A records in GCP Cloud DNS (if using GCP DNS)
    # This requires gcloud CLI and Cloud DNS API enabled
    
    log "Updating GCP Cloud DNS records..."
    
    # Main domain
    gcloud dns record-sets update snailmail.org \
        --rrdatas="$ip" \
        --type=A \
        --zone=snailmail-zone 2>/dev/null || \
    log "Note: Update main A record manually or use DuckDNS"
    
    # Subdomains
    for subdomain in "${SUBDOMAINS[@]}"; do
        gcloud dns record-sets update "$subdomain.snailmail.org" \
            --rrdatas="$ip" \
            --type=A \
            --zone=snailmail-zone 2>/dev/null || true
    done
    
    log "✓ GCP DNS records updated"
}

check_and_update() {
    local current_ip=$(get_current_ip)
    local last_ip=$(get_last_ip)
    
    if [ -z "$current_ip" ]; then
        log "✗ Could not get current IP"
        return 1
    fi
    
    log "Current IP: $current_ip"
    
    if [ "$current_ip" != "$last_ip" ]; then
        log "IP changed: $last_ip → $current_ip"
        
        # Update DuckDNS
        update_duckdns "$current_ip"
        
        # Optionally update GCP DNS (uncomment if using GCP Cloud DNS)
        # update_gcp_dns "$current_ip"
        
        # Save new IP
        save_ip "$current_ip"
        
        # Restart services that might cache IP
        systemctl reload nginx 2>/dev/null || true
        
        log "✓ IP update complete"
    else
        log "IP unchanged: $current_ip"
    fi
}

# ============================================================================
# MAIN
# ============================================================================

# Create log directory
mkdir -p /opt/snailmail/logs

# Check if running as daemon
if [ "$1" = "--daemon" ]; then
    log "Starting IP monitor daemon (checking every ${CHECK_INTERVAL}s)..."
    
    while true; do
        check_and_update
        sleep $CHECK_INTERVAL
    done
else
    # Run once
    check_and_update
fi
