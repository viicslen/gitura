// Folders that only hold tests or test support, at any depth (src/test/java, __tests__, testdata…).
// `spec/` and `fixtures/` are left out: they often hold API specs or seed data; test files in
// them still match by name.
const TEST_DIRS = new Set([
  '__tests__', 'e2e', 'cypress', 'testdata', 'test-data', 'testfixtures', '__fixtures__',
  'mocks', '__mocks__', '__snapshots__', 'androidtest', 'testutil', 'testutils', 'test-utils', 'test_utils',
])
// test/, tests/, unittests/, and .NET test projects such as Acme.Tests/ or Acme.IntegrationTests/.
const TEST_DIR_NAME = /^(.+\.)?(unit|integration|functional|acceptance)?tests?$/i

const TEST_NAMES = [
  // foo.test.ts, foo.spec.jsx, foo.e2e-spec.ts, foo.cy.ts
  /\.(test|spec|e2e|e2e-spec|cy)\.[^.]+$/i,
  // Go, Python, Ruby, Elixir, Dart, Rust, C/C++: foo_test.go, foo_spec.rb, foo_unittest.cc
  /_(test|tests|spec|unittest)\.[^.]+$/i,
  // Python, C, Rust: test_foo.py; Django: tests.py
  /^test_[^/]+\.(py|c|cc|cpp|rs)$/i,
  /^tests\.py$/i,
  // RSpec / Minitest support files
  /^(spec_helper|rails_helper|test_helper)\.rb$/i,
  // JVM, .NET, PHP, Swift: FooTest.java, FooTests.cs, FooIT.kt, FooSpec.groovy, FooTest.php
  /[a-z0-9](Test|Tests|IT|Spec)\.(java|kt|kts|scala|groovy|cs|fs|vb|php|swift|m|mm)$/,
  // Snapshots and test-runner config.
  /\.snap$/i,
  /^(conftest\.py|pytest\.ini|phpunit\.xml(\.dist)?|\.?(jest|vitest|playwright|cypress)\.config\.[^.]+|karma\.conf\.[^.]+)$/i,
  /^((jest|vitest)\.setup|setupTests)\.[^.]+$/i,
]

/** Whether a repo path is a test, test fixture, snapshot or test-runner config, judged by its path alone. */
export function isTestFile(path: string): boolean {
  const parts = path.split('/')
  const name = parts.pop() ?? ''
  return parts.some((dir) => TEST_DIRS.has(dir.toLowerCase()) || TEST_DIR_NAME.test(dir))
    || TEST_NAMES.some((re) => re.test(name))
}
