@wip
Feature: Logout and session termination
  As a staff member or project maintainer
  I want to log out of the web app
  So that my session is terminated and access is revoked

  Scenario Outline: Authenticated user logs out
    Given I am signed in as "<role>" "<github_login>"
    When I click the log out button
    Then my session is terminated
    And I am returned to the sign-in page

    Examples:
      | role       | github_login |
      | staff      | staff-alice  |
      | maintainer | maint-bob    |

  Scenario: Access is blocked after logout
    Given I am signed in as staff "staff-alice"
    And I have logged out
    When I attempt to access a protected page
    Then I am redirected to the sign-in page
