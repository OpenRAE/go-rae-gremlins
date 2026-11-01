# Maintenance plan

This plan makes go-rae-gremlins a well-maintained fork of Gremlins. Each phase is a GitHub milestone.
A phase starts when the phase before it is complete.

Other repositories need correct results from this fork:

- autarchy-ai/coldlight#3 selects go-rae-gremlins for mutation testing.
- OpenRAE/env-packs#557 makes mutation testing on changed lines a merge gate.

The defects in #1 are on the critical path of that work.

## 00 Capture

Exit: The open upstream issues and PRs are in the fork. Nobody merges a copied item before 03 Triage.

- #66 Copy the four upstream PRs that the pre-push guard blocked

## 01 Ready for development

Exit: A trivial change goes from issue to PR, passes CI, and merges into `dev` under Ground Control.

- #65 Audit the workflows for safety and align them with the maintainer's practices and Gremlins
- #67 Enable the workflows of the fork and make the checks of #64 pass
- #68 Move the Sonar analysis to the raesystem organization
- #69 Decide the module path of the fork
- #70 Decide the binary name and the version scheme of the fork
- #71 Decide how the fork takes upstream changes
- #72 Document the build and release process of upstream Gremlins
- #73 Set up the release process of the fork
- #74 Make Ground Control work from issue to merge in this repository

## 02 Understand

Exit: Each review has a record, a baseline, and decisions. The reviews do not fix code.

- #75 Review the development process and merge the upstream conventions with ours
- #76 Model the architecture as code and check designs against the model
- #77 Set the coding standards for strict linting and the Google Go Style rules
- #78 Define the test standard
- #79 Gate changes on mutation testing of changed lines and on regression proof
- #80 Review the code quality
- #81 Review the security
- #82 Review the documentation

## 03 Triage

Exit: Each copied item and #1 has a decision: keep, rework, or close.

- #83 Triage the copied upstream items and #1 against the review results

## 04 Remediate and develop

Exit: The kept work and the remediations are done under the new gates, and a tagged release exists that the dependent repositories can pin.

- #84 Cut a tagged release that the dependent repositories can pin

The 03 Triage milestone adds the remediation and development issues to this milestone.
