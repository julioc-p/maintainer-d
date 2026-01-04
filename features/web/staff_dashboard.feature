@wip
Feature: Staff landing dashboard
  As a staff member
  I want a search box and recently visited lists
  So that I can quickly find and resume work

  Background:
    Given I am signed in as staff

  Scenario Outline: Staff sees global search results for free text queries
    When I view the staff dashboard
    And I search for "<query>"
    Then I see search results that include "<result_type>"
    And I can open the "<result_type>" result "<result_name>"

    Examples:
      | query    | result_type | result_name |
      | alpha    | project     | Alpha       |
      | alice    | maintainer  | alice       |
      | acme     | company     | Acme Corp   |

  Scenario Outline: Staff sees recently visited lists
    Given I have recently visited projects, maintainers, and services
    When I view the staff dashboard
    Then I see a list of recently visited projects
    And I see a list of recently visited maintainers
    And I see a list of recently visited services
    And the recently visited projects include "<project_name>"
    And the recently visited maintainers include "<maintainer_login>"
    And the recently visited services include "<service_name>"

    Examples:
      | project_name | maintainer_login | service_name |
      | Alpha        | alice            | FOSSA        |
      | Beta         | bob              | GitHub       |

  Scenario: Staff sees current foudation stats
    When I view the staff dashboad
    Then I see a summary of the full maintainer count, full project count, graduated project count, incubating project count, sandbox project count
