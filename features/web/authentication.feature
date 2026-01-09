@wip
Feature: Web app authentication via GitHub OIDC
  As a staff member or project maintainer
  I want to sign in with GitHub
  So that I can access the data that I am authorized to see

  Background:
    Given the maintainer-d database contains staff and maintainer GitHub accounts

  Scenario Outline: Staff member signs in with GitHub OIDC
    Given a GitHub account "<github_login>" exists in the database in the staff_members table
    When the user signs in with GitHub OIDC using that account
    Then the user is authenticated
    And the user is authorized as staff

    Examples:
      | github_login |
      | staff-tester  |

  Scenario Outline: Project maintainer signs in with GitHub OIDC
    Given a maintainer GitHub account "<github_login>" exists in the database
    When the user signs in with GitHub OIDC using that account
    Then the user is authenticated
    And the user is authorized as a project maintainer

    Examples:
      | github_login |
      | antonio-example |
      | renee-sample    |
      | diego-placeholder |
      | jun-example       |
      | priya-demo        |


  Scenario Outline: Unknown GitHub account is denied access
    Given no GitHub account "<github_login>" exists in the database
    When the user signs in with GitHub OIDC using that account
    Then the user is denied access
    And the user is shown an unauthorized message

    Examples:
      | github_login |
      | unknown-dev  |
