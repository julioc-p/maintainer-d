Feature: Create new projects
  As a CNCF staff member
  I want to create a new project record when a Project has been admitted to the sandbox repo
  So that maintainer-d can then allow the registration of maintainers with the Project

  Background:
    Given I am signed in as a staff member

  @wip
  Scenario: Create a new project from an onboarding issue
    When I open the Create Project dialog
    And I enter the onboarding issue URL for a sandbox project
    Then the project name is resolved from the onboarding issue title
    And I can provide either a legacy maintainer file or a dot project YAML URL
    And the GitHub org is inferred from the provided file URL
    When I submit the form
    Then a new project record is created

  Scenario: Restrict onboarding issues to cncf/sandbox
    When I enter an onboarding issue URL outside cncf/sandbox
    Then I am prompted to use a cncf/sandbox issue URL
