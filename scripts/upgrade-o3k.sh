#!/bin/bash
#
# O3K Upgrade Script
# Safely upgrades O3K to the latest version with backup and rollback capability
#
# Usage: sudo ./scripts/upgrade-o3k.sh [--version VERSION] [--force] [--no-backup]
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default configuration
O3K_DIR="/opt/o3k"
BACKUP_DIR="/opt/o3k-backups"
CONFIG_FILE="/opt/o3k/config/o3k.yaml"
SYSTEMD_SERVICE="o3k.service"
TARGET_VERSION="latest"
FORCE_UPGRADE=false
NO_BACKUP=false

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Banner
show_banner() {
    cat << "EOF"
╔═══════════════════════════════════════════════════════════╗
║                                                           ║
║   O3K Upgrade Script                                     ║
║   Safely upgrade to the latest version                   ║
║                                                           ║
╚═══════════════════════════════════════════════════════════╝
EOF
    echo ""
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --version)
                TARGET_VERSION="$2"
                shift 2
                ;;
            --force)
                FORCE_UPGRADE=true
                shift
                ;;
            --no-backup)
                NO_BACKUP=true
                shift
                ;;
            --help|-h)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Show help
show_help() {
    cat << EOF
Usage: sudo ./scripts/upgrade-o3k.sh [OPTIONS]

Options:
  --version VERSION   Upgrade to specific version (tag or branch)
                      Default: latest (main branch)
  --force            Force upgrade even if same version
  --no-backup        Skip backup creation (not recommended)
  -h, --help         Show this help message

Examples:
  # Upgrade to latest version
  sudo ./upgrade-o3k.sh

  # Upgrade to specific version
  sudo ./upgrade-o3k.sh --version v0.5.0

  # Force upgrade
  sudo ./upgrade-o3k.sh --force

  # Upgrade without backup (dangerous)
  sudo ./upgrade-o3k.sh --no-backup

Backup location: /opt/o3k-backups/
Rollback: sudo ./upgrade-o3k.sh --version <backup-tag>
EOF
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Check if O3K is installed
check_installation() {
    if [ ! -d "$O3K_DIR" ]; then
        log_error "O3K not found at $O3K_DIR"
        log_error "Please install O3K first or adjust O3K_DIR path"
        exit 1
    fi

    if [ ! -f "$O3K_DIR/bin/o3k" ]; then
        log_error "O3K binary not found at $O3K_DIR/bin/o3k"
        exit 1
    fi

    if [ ! -f "$CONFIG_FILE" ]; then
        log_error "Configuration file not found at $CONFIG_FILE"
        exit 1
    fi

    log_success "O3K installation found at $O3K_DIR"
}

# Get current version
get_current_version() {
    cd "$O3K_DIR"

    # Try to get version from git
    if [ -d .git ]; then
        CURRENT_VERSION=$(git describe --tags --always 2>/dev/null || echo "unknown")
        CURRENT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    else
        CURRENT_VERSION="unknown"
        CURRENT_COMMIT="unknown"
    fi

    log_info "Current version: $CURRENT_VERSION (commit: $CURRENT_COMMIT)"
}

# Check if service is running
check_service_status() {
    if systemctl is-active --quiet "$SYSTEMD_SERVICE"; then
        SERVICE_RUNNING=true
        log_info "O3K service is running"
    else
        SERVICE_RUNNING=false
        log_warning "O3K service is not running"
    fi
}

# Create backup
create_backup() {
    if [ "$NO_BACKUP" = true ]; then
        log_warning "Skipping backup (--no-backup flag)"
        return
    fi

    log_info "Creating backup..."

    # Create backup directory
    mkdir -p "$BACKUP_DIR"

    # Generate backup name with timestamp
    BACKUP_NAME="o3k-backup-$(date +%Y%m%d-%H%M%S)-$CURRENT_COMMIT"
    BACKUP_PATH="$BACKUP_DIR/$BACKUP_NAME"

    # Create backup directory
    mkdir -p "$BACKUP_PATH"

    # Backup binary
    if [ -f "$O3K_DIR/bin/o3k" ]; then
        cp "$O3K_DIR/bin/o3k" "$BACKUP_PATH/o3k-binary"
    fi

    # Backup configuration
    if [ -f "$CONFIG_FILE" ]; then
        cp "$CONFIG_FILE" "$BACKUP_PATH/o3k.yaml"
    fi

    # Backup git info
    cd "$O3K_DIR"
    if [ -d .git ]; then
        git rev-parse HEAD > "$BACKUP_PATH/commit-hash.txt"
        git describe --tags --always > "$BACKUP_PATH/version.txt" 2>/dev/null || true
    fi

    # Create backup metadata
    cat > "$BACKUP_PATH/metadata.txt" <<EOF
Backup Date: $(date)
Version: $CURRENT_VERSION
Commit: $CURRENT_COMMIT
Service Status: $SERVICE_RUNNING
EOF

    log_success "Backup created: $BACKUP_PATH"

    # Keep only last 5 backups
    cd "$BACKUP_DIR"
    ls -t | tail -n +6 | xargs -r rm -rf
    log_info "Kept last 5 backups, removed older ones"
}

# Stop O3K service
stop_service() {
    if [ "$SERVICE_RUNNING" = true ]; then
        log_info "Stopping O3K service..."
        systemctl stop "$SYSTEMD_SERVICE"

        # Wait for service to stop
        for i in {1..10}; do
            if ! systemctl is-active --quiet "$SYSTEMD_SERVICE"; then
                log_success "Service stopped"
                return
            fi
            sleep 1
        done

        log_error "Failed to stop service"
        exit 1
    fi
}

# Pull latest code
update_code() {
    log_info "Updating code from repository..."

    cd "$O3K_DIR"

    # Ensure we're in a git repository
    if [ ! -d .git ]; then
        log_error "$O3K_DIR is not a git repository"
        exit 1
    fi

    # Stash any local changes
    if ! git diff-index --quiet HEAD --; then
        log_warning "Local changes detected, stashing..."
        git stash save "Auto-stash before upgrade $(date)"
    fi

    # Fetch latest changes
    log_info "Fetching latest changes..."
    git fetch origin

    # Determine target
    if [ "$TARGET_VERSION" = "latest" ]; then
        TARGET="origin/main"
        log_info "Upgrading to latest version (main branch)"
    else
        TARGET="$TARGET_VERSION"
        log_info "Upgrading to version: $TARGET_VERSION"
    fi

    # Check if target exists
    if ! git rev-parse "$TARGET" >/dev/null 2>&1; then
        log_error "Version/branch '$TARGET' not found"
        exit 1
    fi

    # Get new version info
    NEW_COMMIT=$(git rev-parse "$TARGET" | cut -c1-7)

    # Check if already at target version
    if [ "$CURRENT_COMMIT" = "$NEW_COMMIT" ] && [ "$FORCE_UPGRADE" = false ]; then
        log_warning "Already at version $TARGET (commit: $NEW_COMMIT)"
        log_info "Use --force to rebuild anyway"

        # Start service if it was running
        if [ "$SERVICE_RUNNING" = true ]; then
            systemctl start "$SYSTEMD_SERVICE"
        fi

        exit 0
    fi

    # Checkout target version
    log_info "Checking out $TARGET..."
    git checkout "$TARGET"
    git pull origin "$(git rev-parse --abbrev-ref HEAD)" 2>/dev/null || true

    log_success "Code updated to $TARGET (commit: $NEW_COMMIT)"
}

# Build new binary
build_binary() {
    log_info "Building new O3K binary..."

    cd "$O3K_DIR"

    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        log_error "Go compiler not found. Install Go first."
        exit 1
    fi

    # Build binary
    if make build > /tmp/o3k-build.log 2>&1; then
        log_success "Binary built successfully"
    else
        log_error "Build failed. See /tmp/o3k-build.log for details"
        cat /tmp/o3k-build.log
        exit 1
    fi

    # Verify binary
    if [ ! -f "$O3K_DIR/bin/o3k" ]; then
        log_error "Binary not found after build"
        exit 1
    fi

    # Get binary version info
    BINARY_SIZE=$(du -h "$O3K_DIR/bin/o3k" | cut -f1)
    log_info "Binary size: $BINARY_SIZE"
}

# Run database migrations
run_migrations() {
    log_info "Running database migrations..."

    cd "$O3K_DIR"

    # Check if migration is needed
    if ./bin/o3k migrate --config "$CONFIG_FILE" --dry-run 2>/dev/null | grep -q "No migrations to run"; then
        log_info "No new migrations needed"
        return
    fi

    # Run migrations
    if ./bin/o3k migrate --config "$CONFIG_FILE" > /tmp/o3k-migrate.log 2>&1; then
        log_success "Database migrations completed"
    else
        log_error "Migration failed. See /tmp/o3k-migrate.log for details"
        cat /tmp/o3k-migrate.log

        # Ask if should rollback
        read -p "Rollback to previous version? [Y/n]: " ROLLBACK
        if [[ "$ROLLBACK" =~ ^[Yy]?$ ]]; then
            perform_rollback
        fi
        exit 1
    fi
}

# Start O3K service
start_service() {
    if [ "$SERVICE_RUNNING" = true ]; then
        log_info "Starting O3K service..."
        systemctl start "$SYSTEMD_SERVICE"

        # Wait for service to start
        sleep 5

        if systemctl is-active --quiet "$SYSTEMD_SERVICE"; then
            log_success "Service started successfully"
        else
            log_error "Service failed to start"
            log_error "Check logs: journalctl -u $SYSTEMD_SERVICE -n 50"
            exit 1
        fi
    fi
}

# Verify upgrade
verify_upgrade() {
    log_info "Verifying upgrade..."

    # Check service status
    if [ "$SERVICE_RUNNING" = true ]; then
        if ! systemctl is-active --quiet "$SYSTEMD_SERVICE"; then
            log_error "Service is not running after upgrade"
            return 1
        fi
    fi

    # Wait for services to be ready
    log_info "Waiting for services to be ready (10 seconds)..."
    sleep 10

    # Test API endpoints if service is running
    if [ "$SERVICE_RUNNING" = true ]; then
        # Source environment if exists
        if [ -f /root/.o3k-env ]; then
            source /root/.o3k-env
        fi

        # Test Keystone endpoint
        if curl -sf "http://localhost:35357/v3" > /dev/null 2>&1; then
            log_success "✓ Keystone API responding"
        else
            log_warning "✗ Keystone API not responding"
            return 1
        fi

        # Test token authentication if OpenStack CLI is available
        if command -v openstack &> /dev/null; then
            if openstack token issue > /dev/null 2>&1; then
                log_success "✓ Authentication working"
            else
                log_warning "✗ Authentication failed"
            fi
        fi
    fi

    log_success "Upgrade verification completed"
    return 0
}

# Perform rollback
perform_rollback() {
    log_warning "Starting rollback..."

    # Find latest backup
    if [ ! -d "$BACKUP_DIR" ]; then
        log_error "No backups found at $BACKUP_DIR"
        exit 1
    fi

    LATEST_BACKUP=$(ls -t "$BACKUP_DIR" | head -n1)

    if [ -z "$LATEST_BACKUP" ]; then
        log_error "No backup available for rollback"
        exit 1
    fi

    log_info "Rolling back to: $LATEST_BACKUP"

    # Stop service
    stop_service

    # Restore binary
    if [ -f "$BACKUP_DIR/$LATEST_BACKUP/o3k-binary" ]; then
        cp "$BACKUP_DIR/$LATEST_BACKUP/o3k-binary" "$O3K_DIR/bin/o3k"
        chmod +x "$O3K_DIR/bin/o3k"
        log_success "Binary restored"
    fi

    # Restore git state if commit hash exists
    if [ -f "$BACKUP_DIR/$LATEST_BACKUP/commit-hash.txt" ]; then
        BACKUP_COMMIT=$(cat "$BACKUP_DIR/$LATEST_BACKUP/commit-hash.txt")
        cd "$O3K_DIR"
        git checkout "$BACKUP_COMMIT" 2>/dev/null || log_warning "Could not restore git state"
    fi

    # Start service
    start_service

    log_success "Rollback completed"
}

# Show upgrade summary
show_summary() {
    echo ""
    echo "╔═══════════════════════════════════════════════════════════╗"
    echo "║                                                           ║"
    echo "║   O3K Upgrade Complete! 🎉                               ║"
    echo "║                                                           ║"
    echo "╚═══════════════════════════════════════════════════════════╝"
    echo ""
    log_info "Upgrade Summary:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "  Previous Version:  $CURRENT_VERSION (commit: $CURRENT_COMMIT)"

    cd "$O3K_DIR"
    NEW_VERSION=$(git describe --tags --always 2>/dev/null || echo "unknown")
    NEW_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

    echo "  New Version:       $NEW_VERSION (commit: $NEW_COMMIT)"
    echo ""
    echo "  Backup Location:   $BACKUP_PATH"
    echo "  Configuration:     $CONFIG_FILE"
    echo "  Service Status:    $(systemctl is-active $SYSTEMD_SERVICE 2>/dev/null || echo "stopped")"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    log_info "Next Steps:"
    echo "  1. Verify services: systemctl status o3k.service"
    echo "  2. Check logs: journalctl -u o3k.service -f"
    echo "  3. Test API: openstack token issue"
    echo ""
    log_info "Rollback (if needed):"
    echo "  systemctl stop o3k.service"
    echo "  cp $BACKUP_PATH/o3k-binary /opt/o3k/bin/o3k"
    echo "  systemctl start o3k.service"
    echo ""
}

# Cleanup old backups
cleanup_backups() {
    if [ -d "$BACKUP_DIR" ]; then
        BACKUP_COUNT=$(ls "$BACKUP_DIR" | wc -l)
        if [ "$BACKUP_COUNT" -gt 5 ]; then
            log_info "Cleaning up old backups (keeping last 5)..."
            cd "$BACKUP_DIR"
            ls -t | tail -n +6 | xargs -r rm -rf
        fi
    fi
}

# Main upgrade flow
main() {
    show_banner

    parse_args "$@"

    check_root
    check_installation
    get_current_version
    check_service_status

    echo ""
    log_info "Starting upgrade process..."
    echo ""

    # Create backup before any changes
    create_backup

    # Stop service
    stop_service

    # Update code
    update_code

    # Build new binary
    build_binary

    # Run migrations
    run_migrations

    # Start service
    start_service

    # Verify upgrade
    if verify_upgrade; then
        show_summary
        cleanup_backups
    else
        log_error "Upgrade verification failed"
        log_warning "Service may be running but API verification failed"
        log_info "Check logs: journalctl -u $SYSTEMD_SERVICE -n 100"

        read -p "Rollback to previous version? [y/N]: " ROLLBACK
        if [[ "$ROLLBACK" =~ ^[Yy]$ ]]; then
            perform_rollback
        fi
        exit 1
    fi
}

# Run main function
main "$@"
