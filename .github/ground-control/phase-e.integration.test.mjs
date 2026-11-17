// Execute the workflow's shell transport and installed CLI against a stateful
// GitHub stand-in. No native authorization or finalizer function is substituted.
import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, writeFileSync, mkdirSync, symlinkSync, rmSync, copyFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve, join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import { execFileSync, spawnSync } from 'node:child_process';
import { test, before, after } from 'node:test';
import { createHash } from 'node:crypto';

const here = dirname(fileURLToPath(import.meta.url));
let job, dispatcher, dispatchStep, steps, asset, installation, utility;
before(async () => {
  assert.ok(process.env.GROUND_CONTROL_TEST_ROOT, 'set GROUND_CONTROL_TEST_ROOT to the pinned installed utility');
  const reference = resolve(process.env.GROUND_CONTROL_TEST_ROOT);
  const requireUtility = createRequire(join(reference, 'package.json'));
  const yaml = requireUtility('js-yaml');
  const workflow = yaml.load(readFileSync(join(here, '../workflows/ground-control-phase-e.yml'), 'utf8'));
  job = workflow.jobs.finalize;
  dispatcher = yaml.load(readFileSync(join(here, '../workflows/ground-control-fork-finalization.yml'), 'utf8'));
  dispatchStep = dispatcher.jobs.dispatch.steps[0];
  steps = Object.fromEntries(job.steps.filter(s => s.name).map(s => [s.name, s]));
  asset = process.env.GROUND_CONTROL_RELEASE_ASSET || resolve(reference, '../..', job.env.GC_ASSET);
  installation = mkdtempSync(join(tmpdir(), 'go-rae-verified-finalizer-'));
  const install = join(installation, 'ground-control');
  mkdirSync(install);
  for (const name of ['package.json', 'package-lock.json']) copyFileSync(join(here, name), join(install, name));
  copyFileSync(asset, join(install, job.env.GC_ASSET));
  assert.equal(createHash('sha256').update(readFileSync(join(install, job.env.GC_ASSET))).digest('hex'), job.env.GC_SHA256);
  execFileSync('npm', ['ci', '--ignore-scripts', '--omit=dev', '--no-audit', '--no-fund', '--userconfig=/dev/null',
    `--globalconfig=${install}/global.npmrc`, `--cache=${installation}/cache`], { cwd: install, stdio: 'pipe' });
  utility = join(install, 'node_modules/grndctl');
  assert.equal(JSON.parse(readFileSync(join(utility, 'package.json'), 'utf8')).version, job.env.GC_RELEASE.slice(1));
});
after(() => { if (installation) rmSync(installation, { recursive: true, force: true }); });
const issue = 1103;
const prNumber = 42;

const pickup = (lane, branch) => `🛠️ Picked up by /${lane} - driver maintainer, branch \`${branch}\`, 2026-11-03T00:00:00.000Z.`;

function fixture({ event = 'pull_request', base = 'dev', head = '1103-delivery', merged = true, lane = 'implement', pickupBranch = head, picked = true, fork = false, untrusted = false, provenancePr = 42, permissions = job.permissions } = {}) {
  const root = mkdtempSync(join(tmpdir(), 'go-rae-phase-e-integration-'));
  const repo = join(root, 'repo');
  const bin = join(root, 'bin');
  const runner = join(root, 'runner');
  for (const path of [repo, bin, join(runner, 'ground-control/node_modules/.bin')]) mkdirSync(path, { recursive: true });
  symlinkSync(join(utility, 'bin/grndctl.js'), join(runner, 'ground-control/node_modules/.bin/grndctl'));
  symlinkSync(utility, join(runner, 'ground-control/node_modules/grndctl'), 'dir');
  const git = (...args) => execFileSync('git', ['-C', repo, ...args], { encoding: 'utf8' }).trim();
  git('init', '-q', '--initial-branch=1103-delivery');
  git('config', 'user.email', 'test@example.com');
  git('config', 'user.name', 'Test');
  writeFileSync(join(repo, 'README'), 'Synthetic delivery\n');
  writeFileSync(join(repo, '.ground-control.yaml'), 'schema_version: 1\nproject: phase-e-fixture\ngithub_repo: example/phase-e-fixture\nworkflow:\n  base_branch: dev\n');
  git('add', '.'); git('commit', '-qm', 'Initialize fixture');
  git('remote', 'add', 'origin', 'https://github.com/example/phase-e-fixture.git');
  const sha = git('rev-parse', 'HEAD');
  writeFileSync(join(repo, 'README'), 'Delivered fixture\n');
  git('add', 'README'); git('commit', '-qm', 'Record fixture merge');
  const mergeSha = git('rev-parse', 'HEAD');
  const transport = join(repo, 'transport/.github/ground-control');
  mkdirSync(transport, { recursive: true });
  for (const name of ['package.json', 'package-lock.json']) copyFileSync(join(here, name), join(transport, name));
  const comment = (id, body) => ({ id, body, user: { login: untrusted && id === 500 ? 'outsider' : 'maintainer', type: 'User' }, author_association: 'MEMBER' });
  const repository = { full_name: 'example/phase-e-fixture', name: 'phase-e-fixture', owner: { login: 'example' } };
  const statePath = join(root, 'state.json');
  writeFileSync(statePath, JSON.stringify({
    pr: { number: prNumber, state: 'closed', merged, merged_at: merged ? '2026-11-03T00:00:00Z' : null,
      merge_commit_sha: mergeSha, title: 'fix: deliver the fixture change', body: `Closes #${issue}`,
      head: { ref: head, sha, repo: fork ? { ...repository, full_name: 'contributor/fork' } : repository }, base: { ref: base, sha, repo: repository } },
    event, permissions, provenancePr, dispatchPermissions: dispatcher.jobs.dispatch.permissions, dispatches: [], asset, calls: [],
    comments: picked ? [comment(500, pickup(lane, pickupBranch))] : [],
    files: [{ filename: 'README', status: 'modified' }],
    closed: false, closes: [],
  }));
  writeFileSync(join(bin, 'gh'), '#!/bin/sh\nexec node "$PHASE_E_GITHUB_STANDIN" "$@"\n', { mode: 0o700 });
  const env = { ...process.env, GC_RELEASE: job.env.GC_RELEASE, GC_ASSET: job.env.GC_ASSET, GC_SHA256: job.env.GC_SHA256, PATH: bin + ':' + process.env.PATH, GH_TOKEN: 'synthetic-installation-token',
    GITHUB_TOKEN: 'synthetic-installation-token', GITHUB_RUN_ID: '7', GITHUB_EVENT_NAME: event,
    GITHUB_REF: event === 'workflow_dispatch' ? 'refs/heads/main' : 'refs/heads/dev', DEFAULT_BRANCH: 'main',
    GITHUB_REPOSITORY: 'example/phase-e-fixture', PR: String(prNumber),
    GITHUB_OUTPUT: join(root, 'output'), RUNNER_TEMP: runner, PHASE_E_FIXTURE_STATE: statePath, PHASE_E_GITHUB_STANDIN: join(here, 'fixtures/github.cjs') };
  return {
    state: () => JSON.parse(readFileSync(statePath, 'utf8')),
    run: (name, extra = {}) => spawnSync('bash', ['-euo', 'pipefail', '-c', (name === dispatchStep.name ? dispatchStep : steps[name]).run], { cwd: name === 'Install the locked utility' ? join(runner, 'ground-control') : repo, env: { ...env, ...extra }, encoding: 'utf8' }),
    resolvedSha: () => readFileSync(env.GITHUB_OUTPUT, 'utf8').trim(),
    mergeSha, runner, repo,
    cleanup: () => rmSync(root, { recursive: true, force: true }),
  };
}

const finalReports = (f) => f.state().comments.filter(c => c.body.includes('gc:final-report'));
const assertUntouched = (f) => {
  assert.equal(f.state().closed, false);
  assert.deepEqual(f.state().closes, []);
  assert.equal(finalReports(f).length, 0);
};

for (const event of ['pull_request', 'workflow_dispatch']) for (const base of ['dev', 'main']) {
  test(`${event}/${base}: actual workflow-to-CLI path finalizes the correct issue and retries without duplicate writes`, () => {
    const f = fixture({ event, base });
    try {
      const resolved = f.run('Resolve the merged delivery');
      assert.equal(resolved.status, 0, resolved.stderr);
      assert.equal(f.resolvedSha(), 'sha=' + f.mergeSha);
      const checkout = job.steps.find(s => s.name === 'Check out the immutable delivered revision');
      assert.equal(checkout.with.ref, '${{ steps.delivery.outputs.sha }}');
      assert.equal(checkout.with['persist-credentials'], false);
      assert.equal(job.steps.find(s => s.name === 'Check out the transport definition').with.ref, '${{ github.workflow_sha }}');
      const first = f.run('Finalize the merged delivery');
      assert.equal(first.status, 0, first.stdout + first.stderr);
      const result = JSON.parse(first.stdout);
      assert.equal(result.status, 'finalized');
      assert.equal(result.issue_number, issue);
      const state = f.state();
      assert.equal(state.closed, true);
      assert.deepEqual(state.closes, ['/repos/example/phase-e-fixture/issues/1103']);
      const reports = finalReports(f);
      assert.equal(reports.length, 1);
      assert.match(reports[0].body, /issue="1103" pr="42"/);
      assert.match(reports[0].body, /gc:finalizer-run pr="42" id="7"/);
      const replay = f.run('Finalize the merged delivery');
      assert.equal(replay.status, 0, replay.stdout + replay.stderr);
      assert.equal(finalReports(f).length, 1);
      assert.equal(f.state().closes.length, 1);
      assert.ok(f.state().calls.some(c => c.endpoint.endsWith('/actions/runs/7')), 'retry must verify real Actions provenance');
    } finally { f.cleanup(); }
  });
}

test('quickfix lane: the pickup record selects the lane and the issue closes', () => {
  const f = fixture({ lane: 'quickfix' });
  try {
    const result = f.run('Finalize the merged delivery');
    assert.equal(result.status, 0, result.stdout + result.stderr);
    assert.equal(JSON.parse(result.stdout).lane, 'quickfix');
    assert.equal(f.state().closed, true);
    assert.equal(finalReports(f).length, 1);
  } finally { f.cleanup(); }
});

test('promotion from dev into main resolves without required checks and is not a delivery', () => {
  const f = fixture({ base: 'main', head: 'dev', picked: false });
  try {
    const resolved = f.run('Resolve the merged delivery');
    assert.equal(resolved.status, 0, resolved.stderr);
    assert.equal(f.resolvedSha(), 'sha=' + f.mergeSha);
    const result = f.run('Finalize the merged delivery');
    assert.equal(result.status, 0, result.stdout + result.stderr);
    assert.equal(JSON.parse(result.stdout).reason, 'not_a_delivery_branch');
    assertUntouched(f);
  } finally { f.cleanup(); }
});

for (const [name, options, reason] of [
  ['no pickup record', { picked: false }, 'no_pickup_record_for_the_branch'],
  ['a pickup record for another branch', { pickupBranch: '1103-other' }, 'no_pickup_record_for_the_branch'],
  ['an untrusted pickup author', { untrusted: true }, 'no_pickup_record_for_the_branch'],
]) {
  test(`actual CLI skips a delivery with ${name} without a final report or closure`, () => {
    const f = fixture(options);
    try {
      const result = f.run('Finalize the merged delivery');
      assert.equal(result.status, 0, result.stdout + result.stderr);
      assert.deepEqual([JSON.parse(result.stdout).status, JSON.parse(result.stdout).reason], ['skipped', reason]);
      assertUntouched(f);
    } finally { f.cleanup(); }
  });
}

test('actual workflow and CLI refuse an unmerged delivery without a final report or closure', () => {
  const f = fixture({ merged: false });
  try {
    assert.notEqual(f.run('Resolve the merged delivery').status, 0);
    const result = f.run('Finalize the merged delivery');
    assert.equal(result.status, 1, result.stdout + result.stderr);
    assert.equal(JSON.parse(result.stdout).error, 'phase_e_pr_not_merged');
    assertUntouched(f);
  } finally { f.cleanup(); }
});

test('declared issue-write permission is necessary: a read-only installation cannot report or close', () => {
  const f = fixture({ permissions: { ...job.permissions, issues: 'read' } });
  try {
    const result = f.run('Finalize the merged delivery');
    assert.equal(result.status, 1);
    assertUntouched(f);
  } finally { f.cleanup(); }
});

test('actual download step rejects a tampered release before installation', () => {
  const f = fixture();
  try {
    const state = f.state();
    const tampered = join(f.runner, 'tampered.tgz');
    writeFileSync(tampered, 'tampered package');
    state.asset = tampered;
    writeFileSync(join(f.runner, '../state.json'), JSON.stringify(state));
    assert.notEqual(f.run('Download and verify Ground Control').status, 0);
    assert.equal(f.state().closed, false);
  } finally { f.cleanup(); }
});

test('actual verified download and locked-install steps install the pinned executable', () => {
  const f = fixture();
  try {
    const downloaded = f.run('Download and verify Ground Control');
    assert.equal(downloaded.status, 0, downloaded.stderr);
    const installed = f.run('Install the locked utility');
    assert.equal(installed.status, 0, installed.stdout + installed.stderr);
    const bin = join(f.runner, 'ground-control/node_modules/.bin/grndctl');
    assert.equal(execFileSync(bin, ['--version'], { cwd: f.repo, encoding: 'utf8' }).trim(), job.env.GC_RELEASE.slice(1));
  } finally { f.cleanup(); }
});

test('mismatched Actions provenance fails before the CLI can report or close', () => {
  const f = fixture({ provenancePr: 99 });
  try {
    assert.notEqual(f.run('Finalize the merged delivery').status, 0);
    assertUntouched(f);
  } finally { f.cleanup(); }
});

for (const base of ['dev', 'main']) {
  test(`fork/${base}: trusted dispatcher starts the supported workflow-dispatch finalizer`, () => {
    const f = fixture({ event: 'workflow_dispatch', fork: true, base });
    try {
      const dispatched = f.run(dispatchStep.name, { GH_TOKEN: 'synthetic-dispatch-token' });
      assert.equal(dispatched.status, 0, dispatched.stderr);
      assert.deepEqual(f.state().dispatches, [{ ref: 'main', inputs: { pr: '42' } }]);
      assert.equal(f.state().closed, false, 'dispatcher has no closure authority');
      assert.equal(f.run('Resolve the merged delivery').status, 0);
      const finalized = f.run('Finalize the merged delivery');
      assert.equal(finalized.status, 0, finalized.stdout + finalized.stderr);
      assert.equal(f.state().closed, true);
      assert.equal(finalReports(f).length, 1);
    } finally { f.cleanup(); }
  });
}
