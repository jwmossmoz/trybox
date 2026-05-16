# Trybox macOS Scratch Image Build Log

Status as of 2026-05-15 22:30 EDT: macos15 and macos26 scratch proofs are
complete. Both images were built from IPSW with `ci/build-local-macos-image.sh`,
completed the install, Recovery SIP-disable, and finalize/TCC phases
unattended, and were verified by fresh `trybox run -- sw_vers` executions.
The local Tart Packer plugin fix was upstreamed as
https://github.com/cirruslabs/packer-plugin-tart/pull/235.

This file is the live handoff log for issue 23. Update it after every failed
build attempt and mirror the latest state into the issue body.

## Goal

Prove that Trybox can build local Tart images from scratch with:

```sh
ci/build-local-macos-image.sh --target macos15-arm64
ci/build-local-macos-image.sh --target macos26-arm64
```

Success means each command completes all build phases, leaves a local Tart image
named for the target, and the image can boot far enough for Trybox to run a
basic command against it.

## Current Build Shape

`ci/build-local-macos-image.sh` now runs three Packer phases against one
temporary build VM:

1. `ci/macos/packer/trybox.pkr.hcl`
   - Creates a clean VM with Tart `from_ipsw`.
   - Drives Setup Assistant.
   - Enables SSH, auto-login, sudo without password, Screen Sharing, and the
     Tart guest agent.
   - Installs Command Line Tools and non-TCC provisioning scripts.
2. `ci/macos/packer/trybox-disable-sip.pkr.hcl`
   - Boots Recovery and runs `csrutil disable`.
   - This is required before writing the system TCC database on macOS 15+.
3. `ci/macos/packer/trybox-finalize.pkr.hcl`
   - Boots normally with SIP disabled.
   - Runs `030-tcc-ui-automation.sh`.

This mirrors the useful part of the Cirrus Labs image pattern without consuming
their published base images as the Trybox base.

## Validated Locally

These checks passed after the final macOS 15 and macOS 26 proof builds:

```sh
tart list

packer validate -var 'vm_name=trybox-build-validate' \
  -var 'ipsw=/Users/jwmoss/Library/Caches/trybox/ipsw/UniversalMac_15.6.1_24G90_Restore.ipsw' \
  -var 'cpu_count=8' -var 'memory_gb=16' -var 'disk_size_gb=200' \
  -var 'display=1920x1200' -var 'headless=false' \
  -var 'setup_flow=macos15' \
  ci/macos/packer/trybox.pkr.hcl

packer validate -var 'vm_name=trybox-build-validate' \
  -var 'ipsw=/Users/jwmoss/Library/Caches/trybox/ipsw/UniversalMac_26.4_25E246_Restore.ipsw' \
  -var 'cpu_count=8' -var 'memory_gb=16' -var 'disk_size_gb=200' \
  -var 'display=1920x1200' -var 'headless=false' \
  -var 'setup_flow=macos26' \
  ci/macos/packer/trybox.pkr.hcl

packer validate -var 'vm_name=trybox-build-validate' \
  ci/macos/packer/trybox-disable-sip.pkr.hcl

packer validate -var 'vm_name=trybox-build-validate' \
  ci/macos/packer/trybox-finalize.pkr.hcl

shellcheck ci/build-local-macos-image.sh ci/macos/provision.d/*.sh
go test ./...
```

## Findings

- CirrusLabs templates are close references, not exact matches. The Trybox
  template must be checked against the actual IPSW Setup Assistant flow.
- `ssh: connect to host ... no route to host` is documented by Cirrus as a
  macOS Sequoia host Local Network privacy issue. It only matters once Packer
  reaches SSH. It is not the current blocker.
- `display=1920x1200` can appear as `3840x2400` in Packer logs because the
  guest exposes a Retina backing framebuffer. The host 4K display is not
  changing the template geometry.
- macOS 15.6.1 Setup Assistant is brittle when driven by OCR clicks. The
  current template uses the Cirrus-style VoiceOver keyboard path for the most
  fragile Apple Account, Terms, and Location Services controls.
- The eighteenth build proved that host Local Network privacy is not blocking
  this host: after manual Setup Assistant recovery, Packer connected by SSH to
  the guest at `192.168.64.60` and started the provisioning scripts.
- The retained eighteenth build VM proved phase 2 had not disabled SIP:
  `csrutil status` was still `enabled` after the failed finalize step.
- Recovery on this macOS 15.6.1 image opens Terminal with
  `Shift+Command+T`, not `Command+T`, and `csrutil disable` prompts for both
  authorized user and password. After manually following that flow, phase 3
  completed and reported `System Integrity Protection status: disabled`.
- The nineteenth build, after manual Setup Assistant recovery, completed all
  three phases. That proves the automated Recovery SIP-disable phase now works
  and that TCC finalization succeeds when SIP is actually disabled.
- The twenty-second probe isolated the late Setup Assistant failure to Tart
  plugin OCR click behavior rather than bad coordinates. `vncdotool` clicked
  Packer's exact OCR coordinate for `Continue` on `Choose Your Look` and
  advanced the page. The installed `packer-plugin-tart` v1.20.0 source sends a
  VNC mouse-down for `<click ...>` but did not send a matching mouse-up. A local
  patched plugin that releases the mouse at the same coordinate is now installed
  for the next proof run.
- The twenty-third macOS 15 scratch build completed all three phases unattended
  with the local Tart plugin mouse-up patch. A fresh repo VM cloned from the new
  `trybox-macos15-arm64-image` and `trybox run -- sw_vers` reported macOS
  15.6.1 build 24G90.
- The Tart Packer plugin fix is
  https://github.com/cirruslabs/packer-plugin-tart/pull/235. The issue was
  that `<click ...>` sent only a VNC left-button press. Adding a release event
  after the press makes OCR clicks behave like normal mouse clicks.
- macOS 26.4 Setup Assistant is not compatible with the macOS 15 keyboard
  sequence. The observed early page order is Country/Region, Transfer Your
  Data, Written and Spoken Languages, Accessibility, Data & Privacy, Create a
  Mac Account, Sign In to Your Apple Account, and Terms and Conditions. Apple
  Account skip is also different: open `Other Sign-In Options`, choose `Sign in
  Later in Settings`, then confirm `Skip`.
- The second macOS 26 scratch probe showed that `Sign in Later in Settings`
  is more reliable by keyboard after opening `Other Sign-In Options`
  (`Down`, `Down`, `Return`) than by direct OCR click. After Terms, macOS 26.4
  adds `Age Range`; choose `Adult`.
- The second macOS 26 scratch probe also showed that Location Services is a
  checkbox plus `Continue`, followed by a `Don't Use` confirmation, then
  `Select Your Time Zone`. Later, macOS 26.4 inserts `Your Mac is Ready for
  FileVault`; reusable base images should choose `Not Now`.
- The third macOS 26 scratch build reached FileVault unattended. Choosing
  `Not Now` opens an additional confirmation sheet; the unattended flow must
  click that sheet's `Continue` button before waiting for `Choose Your Look`.
- The fourth macOS 26 scratch build reached the final welcome screen
  unattended. Unlike macOS 15, OCR observed lowercase `welcome` and `Get
  Started`, not `Welcome to Mac`; the macOS 26 flow should click `Get Started`.
- The fifth macOS 26 scratch build completed all three phases unattended with
  the macOS 26-specific Setup Assistant path. A fresh repo VM cloned from the
  new `trybox-macos26-arm64-image` and `trybox run -- sw_vers` reported macOS
  26.4 build 25E246.

## Failure Log

| Attempt | Target | Result | Evidence | Follow-up |
| --- | --- | --- | --- | --- |
| Initial one-phase build | macos15-arm64 | Failed at `030-tcc-ui-automation.sh`: system TCC.db was readonly under SIP. | Issue 23 original body. | Split the build into install, disable-SIP, and finalize phases. |
| Scratch build with OCR Setup Assistant clicks | macos15-arm64 | Stuck on Apple Account. OCR saw `Set Up Later` but the click did not activate it. | `/tmp/trybox-packer-macos15-scratch-thirteenth.log`; VNC showed the Apple Account page still focused on the email field. | Switched Apple Account skip to VoiceOver/keyboard path. |
| Same scratch build after manual VNC click | macos15-arm64 | Stuck in Terms. Final `^Agree$` OCR click matched text inside the license body instead of the modal button. | `/tmp/trybox-packer-macos15-scratch-thirteenth.log`; VNC coordinate click advanced it. | Switched Terms flow to keyboard activation. |
| Same scratch build after manual Terms recovery | macos15-arm64 | Location Services branch did not behave consistently; a StocksWidget dialog also appeared while manually recovering. | `/tmp/trybox-packer-macos15-scratch-thirteenth.log`. | Let VoiceOver path drive Location Services and force timezone later from Terminal. |
| Fourteenth scratch build | macos15-arm64 | Got past Apple Account and Terms. Failed because template waited for `Don.t Use`, but VoiceOver path reached Analytics instead. | `/tmp/trybox-packer-macos15-scratch-fourteenth.log`. | Removed `Don.t Use` and manual Time Zone waits; wait for Analytics after Location Services. |
| Fifteenth scratch build | macos15-arm64 | Interrupted during IPSW install before reaching Setup Assistant. | Process list showed `tart create --from-ipsw`; cancelled cleanly. | Restart after updating this log and issue 23. |
| Sixteenth scratch build | macos15-arm64 | Reached Analytics, Screen Time, Siri, Choose Your Look, and Update Mac Automatically with manual VNC recovery. The unattended flow was wrong at Screen Time because `Set Up Later` was visible but disabled; it was wrong at Siri because clicking `Enable Ask Siri` introduced extra voice selection and Siri improvement pages. The run was cancelled after collecting the page-order evidence. | `/tmp/trybox-packer-macos15-scratch-sixteenth.log`; VNC screenshots captured under `/tmp/trybox-screen-time.png`, `/tmp/trybox-siri.png`, `/tmp/trybox-siri-voice.png`, `/tmp/trybox-current.png`, and `/tmp/trybox-look-current.png`. | Use `Continue` on Screen Time and leave Ask Siri disabled by clicking `Continue` directly on Siri. Restart from scratch. |
| Seventeenth scratch build | macos15-arm64 | Got back to Terms, then stalled because the template waited for the full `SOFTWARE LICENSE AGREEMENT FOR macOS` OCR text while Packer only observed the truncated heading `SOFTWARE LICENSE AGRE`. The run was cancelled after confirming the stuck wait. | `/tmp/trybox-packer-macos15-scratch-seventeenth.log`. | Relax the modal wait to `SOFTWARE LICENSE AGRE` and restart from scratch. |
| Eighteenth scratch build | macos15-arm64 | Reached SSH and provisioning only after manual VNC recovery. Unattended flow still failed at Analytics, Screen Time, Siri/Improve Siri, and Choose Your Look because OCR `Continue` clicks left VoiceOver focused on text/window elements. | `/tmp/trybox-packer-macos15-scratch-eighteenth.log`; live VNC on `127.0.0.1:57728` showed manual recovery paths. | Patch Analytics to keyboard advance, Screen Time to `Set Up Later`, Siri to the Improve Siri `Not Now` branch, and Choose Your Look to repeat Continue. Restart from scratch after this probe finishes or is cancelled. |
| Same eighteenth probe, phase 3 | macos15-arm64 | Phase 1 and phase 2 completed, then finalize failed over SSH with `unable to open database "/Library/Application Support/com.apple.TCC/TCC.db": authorization denied`. | `/tmp/trybox-packer-macos15-scratch-eighteenth.log`; Packer connected to SSH at `192.168.64.60`, then `030-tcc-ui-automation.sh` failed. | Retained VM showed SIP was still enabled. Patch the recovery shortcut to `Shift+Command+T` and send both `admin` prompts. |
| Retained eighteenth VM manual SIP check | macos15-arm64 | Manual Recovery flow disabled SIP, then `trybox-finalize.pkr.hcl` completed successfully. | `/tmp/trybox-packer-retained-finalize-after-manual-sip.log`; finalizer printed `System Integrity Protection status: disabled`. | Restart a clean scratch build to prove the patched recovery automation disables SIP unattended. |
| Nineteenth scratch build | macos15-arm64 | Completed all three phases only after manual VNC recovery, so it is not an unattended proof. The unattended flow stalled on Siri because VoiceOver focus landed on the decorative image and OCR `Continue` did not activate the button. After manual recovery, the late OCR clicks also needed VoiceOver disabled, and `Welcome to Mac` required Return because OCR did not expose its lower continue control. | `/tmp/trybox-packer-macos15-scratch-nineteenth.log`; VNC was `127.0.0.1:57733`; final output was `built local trybox image: trybox-macos15-arm64-image`. | Patch the late Setup Assistant sequence to turn VoiceOver off at Siri, click `Choose For Me`, click `Not Now`, and use Return on `Welcome to Mac`. Restart from scratch for an unattended proof. |
| Twentieth scratch build | macos15-arm64 | Reached Screen Time unattended, which proves the Apple Account, Terms, Location Services, and Analytics path now works. It still stalled at Screen Time: VoiceOver was active, the OCR click on `Set Up Later` selected a text element instead of activating the button, and Packer waited for `Siri` while the VM remained on Screen Time. The run was cancelled cleanly. | `/tmp/trybox-packer-macos15-scratch-twentieth.log`; VNC was `127.0.0.1:57736`; screenshot `/tmp/trybox-macos15-twentieth-current.png` showed the VoiceOver overlay `You are currently on a text element.` | Move the VoiceOver-off shortcut before the Screen Time `Set Up Later` click, then restart from scratch. |
| Twenty-first scratch build | macos15-arm64 | Reached Screen Time unattended again. Moving the VoiceOver toggle earlier removed the worst overlay behavior, but `Set Up Later` still did not advance the page. The active, reliable control on this page is `Continue`. The run was cancelled cleanly while Packer was waiting for `Siri`. | `/tmp/trybox-packer-macos15-scratch-twentyfirst.log`; VNC was `127.0.0.1:57737`; screenshot `/tmp/trybox-macos15-twentyfirst-current.png` showed Screen Time with `Continue` active and `Set Up Later` greyed. | Keep VoiceOver off at Screen Time, but click `Continue` instead of `Set Up Later`. Restart from scratch. |
| Twenty-second scratch build | macos15-arm64 | Reached Screen Time and late Setup Assistant unattended, but still required manual VNC recovery, so it is not an unattended proof. `Screen Time` advanced with the active `Continue` button. `Siri`, `Select a Siri Voice`, and `Improve Siri & Dictation` exposed that OCR clicks were focusing labels/text or only focusing buttons instead of reliably activating them. `Choose Your Look` stalled after Packer clicked the `Continue` OCR coordinate. A `vncdotool` click at Packer's exact coordinate `(2608, 1738)` advanced the page, which points to the Tart plugin click implementation. Source inspection confirmed `<click ...>` sends mouse-down without mouse-up. | `/tmp/trybox-packer-macos15-scratch-twentysecond.log`; VNC was `127.0.0.1:57738`; local plugin backup is `~/.config/packer/plugins/github.com/cirruslabs/tart/packer-plugin-tart_v1.20.0_x5.0_darwin_arm64.upstream-backup`; patched plugin SHA256 is `f8894b7a90d0338e087ecf4fefeb1432804f60ca5ab5434828e2f6ad54116a31`. | Install a local patched `packer-plugin-tart` that sends mouse-down and mouse-up for OCR clicks, then restart from scratch. |
| Twenty-third scratch build | macos15-arm64 | Success. Completed install, Recovery SIP-disable, and finalize/TCC phases unattended from the 15.6.1 IPSW. Late Setup Assistant OCR clicks advanced cleanly with the local Tart plugin mouse-up patch. Fresh Trybox verification succeeded after destroying the old repo-bound workspace VM so Trybox cloned from the new base image. | `/tmp/trybox-packer-macos15-scratch-twentythird.log`; final image `trybox-macos15-arm64-image`; `TRYBOX_TARGET=macos15-arm64 go run ./cmd/trybox run -- sw_vers` returned `ProductVersion: 15.6.1`, `BuildVersion: 24G90`, exit 0. | macos15 proof complete. Move to macos26 scratch proof. |
| First macOS 26 scratch build | macos26-arm64 | Failed/stalled during Setup Assistant and is not an unattended proof. The macOS 15 early keyboard sequence did not advance macOS 26 past Country/Region, so Packer eventually waited for `Email or Phone Number` while OCR still showed `Select Your Country or Region`. Manual VNC recovery mapped the macOS 26 page order through Terms and showed the Apple Account skip branch differs from macOS 15: `Other Sign-In Options` -> `Sign in Later in Settings` -> `Skip`. The Packer process was interrupted after probing. | `/tmp/trybox-packer-macos26-scratch-first.log`; VNC was `127.0.0.1:57742`; screenshots include `/tmp/trybox-macos26-after-transfer-new.png`, `/tmp/trybox-macos26-account-current.png`, `/tmp/trybox-macos26-after-signin-later2.png`, and `/tmp/trybox-macos26-after-skip2.png`. | Add a macOS 26-specific install template or setup-flow variable that uses OCR clicks for the observed macOS 26 pages and does not enable VoiceOver during early setup. Restart from scratch. |
| Second macOS 26 scratch build | macos26-arm64 | Failed/stalled during Setup Assistant and is not an unattended proof. The new macOS 26 setup flow got through account creation and Terms with manual recovery at Apple Account skip. The direct OCR click on `Sign in Later in Settings` did not open the skip confirmation; the keyboard menu path did. After Terms, manual VNC recovery mapped the new `Age Range` page, the Location Services `Continue` plus `Don't Use` confirmation, the `Select Your Time Zone` page, and the `Your Mac is Ready for FileVault` page before `Choose Your Look`. The Packer process was cancelled after collecting the page-order evidence. | `/tmp/trybox-packer-macos26-scratch-second.log`; VNC was `127.0.0.1:57743`; screenshots include `/tmp/trybox-macos26-second-age-range.png`, `/tmp/trybox-macos26-second-timezone.png`, and `/tmp/trybox-macos26-second-filevault.png`. | Patch the macOS 26 setup flow with the keyboard menu skip, Age Range, Location Services confirmation, Time Zone, and FileVault pages. Validate templates and restart from scratch. |
| Third macOS 26 scratch build | macos26-arm64 | Failed/stalled during Setup Assistant and is not an unattended proof. The run reached FileVault unattended, proving the patched Apple Account skip, Age Range, Location Services confirmation, Time Zone, Analytics, Screen Time, Siri, Siri Voice, and Improve Siri sequence worked. It stalled after clicking `Not Now` on `Your Mac is Ready for FileVault` because macOS 26.4 opens a second confirmation sheet asking whether to continue without FileVault. Packer was still waiting for `Choose Your Look`. | `/tmp/trybox-packer-macos26-scratch-third.log`; VNC was `127.0.0.1:57744`; OCR observed `Are you sure you want to continue without FileVault?` with `Cancel` and `Continue`. | Add a `Continue` click after FileVault `Not Now`, validate the templates, and restart from scratch. |
| Fourth macOS 26 scratch build | macos26-arm64 | Failed/stalled during Setup Assistant and is not an unattended proof. The run reached the final welcome screen unattended, proving the FileVault `Not Now` plus confirmation `Continue` sequence works. It stalled because the template waited for `Welcome to Mac`, but OCR repeatedly observed lowercase `welcome` and `Get Started`. | `/tmp/trybox-packer-macos26-scratch-fourth.log`; VNC was `127.0.0.1:57745`; OCR observed `welcome` and `Get Started`. | Change the macOS 26 final screen to wait for `welcome` and click `Get Started`. Validate templates and restart from scratch. |
| Fifth macOS 26 scratch build | macos26-arm64 | Success. Completed install, Recovery SIP-disable, and finalize/TCC phases unattended from the 26.4 IPSW. Fresh Trybox verification succeeded after destroying the old repo-bound workspace VM so Trybox cloned from the new base image. | `/tmp/trybox-packer-macos26-scratch-fifth.log`; final image `trybox-macos26-arm64-image`; `TRYBOX_TARGET=macos26-arm64 go run ./cmd/trybox run -- sw_vers` returned `ProductVersion: 26.4`, `BuildVersion: 25E246`, exit 0. | macos26 proof complete. Run final validation and repo checks. |

## Current Status

### macos15-arm64

Current proof state:

1. Apple Account skip by keyboard.
2. Terms agreement by keyboard.
3. Location Services disabled by keyboard.
4. Analytics page reached next.
5. Analytics advances by the VoiceOver keyboard path instead of OCR `Continue`.
6. Screen Time keeps VoiceOver disabled and clicks the active `Continue`
   button.
7. Siri follows the observed default-enabled path, clicks `Choose For Me`,
   selects `Not Now` on Improve Siri & Dictation, then continues.
8. Choose Your Look and Update Mac Automatically use OCR `Continue` clicks.
   Those clicks need the local Tart plugin mouse-up patch described above.
9. Terminal config enables SSH and Screen Sharing, then Packer should connect by
   SSH and run provisioners.
10. The current recovery template opens Terminal with `Shift+Command+T` and
    sends both the authorized-user and password prompts for `csrutil disable`.
11. With SIP actually disabled, the TCC finalization phase passed on the
    retained build VM.

Proof command:

```sh
PACKER_LOG=1 PACKER_LOG_PATH=/tmp/trybox-packer-macos15-scratch-twentythird.log \
  ci/build-local-macos-image.sh \
    --target macos15-arm64 \
    --ipsw "$HOME/Library/Caches/trybox/ipsw/UniversalMac_15.6.1_24G90_Restore.ipsw" \
    --replace \
    --build-vm trybox-build-macos15-scratch-twentythird \
    --keep-on-failure

go run ./cmd/trybox destroy --target macos15-arm64 --repo /Users/jwmoss/github_moz/trybox
TRYBOX_TARGET=macos15-arm64 go run ./cmd/trybox run -- sw_vers
```

Result: `ProductVersion: 15.6.1`, `BuildVersion: 24G90`, exit 0.

### macos26-arm64

The IPSW is already cached locally:

```text
$HOME/Library/Caches/trybox/ipsw/UniversalMac_26.4_25E246_Restore.ipsw
```

Current implemented macOS 26.4 Setup Assistant path:

1. Country/Region: United States is selected; click `Continue`.
2. Transfer Your Data to This Mac: choose `Set up as new`, then `Continue`.
3. Written and Spoken Languages: click `Continue`.
4. Accessibility: click `Not Now`.
5. Data & Privacy: click `Continue`.
6. Create a Mac Account: create `admin`/`admin`, uncheck Apple Account password
   reset, then click `Continue`.
7. Sign In to Your Apple Account: open `Other Sign-In Options`, choose `Sign in
   Later in Settings` by keyboard menu selection, then confirm `Skip`.
8. Terms and Conditions: click `Agree`, then confirm the modal `Agree`.
9. Age Range: choose `Adult`.
10. Location Services: click `Continue`, then confirm `Don't Use`.
11. Time Zone: click `Continue`; final timezone is forced to UTC later from
    Terminal.
12. Analytics, Screen Time, Siri, Siri Voice, and Improve Siri follow the
    broad macOS 15 late path.
13. FileVault: click `Not Now`, then confirm `Continue` on the no-FileVault
    sheet.
14. Choose Your Look and Update Mac Automatically follow.
15. Final welcome screen: wait for lowercase `welcome`, click `Get Started`,
    then run Terminal configuration.

Proof command:

```sh
PACKER_LOG=1 PACKER_LOG_PATH=/tmp/trybox-packer-macos26-scratch-fifth.log \
  ci/build-local-macos-image.sh \
    --target macos26-arm64 \
    --ipsw "$HOME/Library/Caches/trybox/ipsw/UniversalMac_26.4_25E246_Restore.ipsw" \
    --replace \
    --build-vm trybox-build-macos26-scratch-fifth \
    --keep-on-failure
```

Then verify:

```sh
go run ./cmd/trybox destroy --target macos26-arm64 --repo /Users/jwmoss/github_moz/trybox
TRYBOX_TARGET=macos26-arm64 go run ./cmd/trybox run -- sw_vers
```

Result: `ProductVersion: 26.4`, `BuildVersion: 25E246`, exit 0.

## Final Validation

Final checks completed after both proof builds:

```sh
tart list
packer validate -var 'setup_flow=macos15' ci/macos/packer/trybox.pkr.hcl
packer validate -var 'setup_flow=macos26' ci/macos/packer/trybox.pkr.hcl
packer validate ci/macos/packer/trybox-disable-sip.pkr.hcl
packer validate ci/macos/packer/trybox-finalize.pkr.hcl
shellcheck ci/build-local-macos-image.sh ci/macos/provision.d/*.sh
go test ./...
```

Results:

- `tart list` shows `trybox-macos15-arm64-image` and
  `trybox-macos26-arm64-image`.
- All four Packer validations returned `The configuration is valid.`
- `shellcheck` returned exit 0.
- `go test ./...` passed.
