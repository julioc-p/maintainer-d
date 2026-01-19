@wip
Feature: Role-based access to project data
  As a staff member or project maintainer
  I want to access project data based on my role
  So that data visibility matches permissions

  Background:
    Given the maintainer-d database contains staff, maintainers, and projects

  Scenario Outline: Staff can access all projects and maintainers
    Given I am signed in as staff
    When I view the projects list
    Then I can see all projects
    And I can see all maintainers
    And project "<project_name>" is visible
    And maintainer "<maintainer_login>" is visible
    And I can see all services the project is setup on

    Examples:
      | project_name | maintainer_login |
      | Alpha        | alice            |
      | Beta         | bob              |

  Scenario Outline: Maintainer can access their project and team records
    Given I am signed in as a maintainer for project "<project_name>"
    When I view project "<project_name>"
    Then I can see project "<project_name>" data
    And I can see maintainer records for project "<project_name>"
    And maintainer "<maintainer_login>" is visible
    And I can see all services the project is setup on

    Examples:
      | project_name | maintainer_login |
      | Alpha        | alice            |
      | Alpha        | charlie          |

  Scenario: Maintainer can access other projects
    Given I am signed in as a maintainer for project "Alpha"
    When I attempt to view project "Beta"
    Then I can see project "Beta" data
