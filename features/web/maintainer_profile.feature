Feature: Maintainer profile page
  As an authenticated user
  I want to open a maintainer profile
  So I can see full maintainer details

  Scenario: Navigate from project card to maintainer card
    Given I am signed in as staff
    When I search for "Antonio"
    And I click on maintainer "Antonio Example"
    Then I see the maintainer card for "Antonio Example"
