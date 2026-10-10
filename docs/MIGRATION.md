# Standalone repository migration

RevenueLeak OS now lives at https://github.com/savisaluwadana/RevenueLeak-OS.

The v0.1 pilot was extracted from `experiments/revenueleak-os/` in Property Management OS at commit `16c03824904545c6c75149bdc3d271eb1e4449f2`, including the executable launcher fix from https://github.com/RealEstateSassApplication/property-management-OS/pull/22. That PR was closed without merging. No Property OS application code, credentials, runtime datastore, or customer data is copied here.

The application is at the repository root. Run `./start-demo.sh` here; CI now uses root paths. Docker, Compose, samples, security notes, architecture and roadmap are included. For an existing pilot, back up its `data/revenueleak.json` before copying that file into this repository's data directory. Never commit that file. This is a source-code extraction, not a transfer of the original Git history or a hosted data migration.
