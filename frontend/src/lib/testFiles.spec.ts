import { describe, it, expect } from 'vitest'
import { isTestFile } from './testFiles'

describe('isTestFile', () => {
  it.each([
    'internal/github/pr_test.go',
    'frontend/src/lib/markdown.spec.ts',
    'src/components/Button.test.tsx',
    'src/__tests__/app.js',
    'web/e2e/login.e2e-spec.ts',
    'cypress/e2e/home.cy.ts',
    'pkg/foo/test_utils.py',
    'tests/Feature/Listing/ListingBatchControllerTest.php',
    'app/Support/QueueTest.php',
    'src/test/java/com/acme/OrderServiceTest.java',
    'src/main/kotlin/OrderServiceIT.kt',
    'Acme.Tests/OrderTests.cs',
    'spec/models/user_spec.rb',
    'test/my_app_test.exs',
    'lib/widget_test.dart',
    'base/strings_unittest.cc',
    'internal/github/testdata/pr.json',
    'frontend/src/lib/__fixtures__/coderabbit.md',
    'src/__snapshots__/view.test.ts.snap',
    'conftest.py',
    'phpunit.xml.dist',
    'frontend/vitest.config.ts',
    'app/src/androidTest/java/ExampleTest.kt',
    'Acme.Tests/Helpers/OrderBuilder.cs',
    'Acme.IntegrationTests/Setup.cs',
    'polls/tests.py',
    'pkg/example_test.py',
    'src/test_example.rs',
    'internal/testutil/server.go',
    'src/mocks/api.ts',
    'jest.setup.ts',
    'src/setupTests.js',
    'spec/rails_helper.rb',
    'test/test_helper.rb',
  ])('treats %s as a test file', (path) => {
    expect(isTestFile(path)).toBe(true)
  })

  it.each([
    'app/Actions/Listing/Listings/BatchClearListingErrors.php',
    'src/contest/ContestService.java',
    'app/Http/Controllers/LatestController.php',
    'src/attestation.ts',
    'internal/github/pr.go',
    'docs/testing.md',
    'src/lib/testFiles.ts',
    'config/queue.php',
    'protest.py',
    'api/spec/openapi.yaml',
    'src/specs/order.schema.json',
    'app/fixtures/initial_data.json',
    'src/latest/index.ts',
    'src/contests/Contest.java',
  ])('treats %s as production code', (path) => {
    expect(isTestFile(path)).toBe(false)
  })
})
