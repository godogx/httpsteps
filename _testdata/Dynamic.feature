Feature: Dynamic data is used in steps

  Scenario: Creating user and making an order
    When I request HTTP endpoint with method "POST" and URI "/user"
    And I request HTTP endpoint with body
    """json
    {"name": "John Doe"}
    """

    # Undefined variable infers its value from the actual data on first encounter.
    Then I should have response with body
    """json5
    {
      // Capturing dynamic user id as $user_id variable.
     "id":"$user_id",
     "user": "$user",
     "name": "John Doe",
     // Ignoring other dynamic values.
     "created_at":"<ignore-diff>","updated_at": "<ignore-diff>"
    }
    """

    # Creating an order for that user with $user_id.
    When I request HTTP endpoint with method "POST" and URI "/order/$user_id/?user_id=$user_id"
    And I request HTTP endpoint with header "X-UserId: $user_id"
    And I request HTTP endpoint with cookie "user_id: $user_id"
    And I request HTTP endpoint with body
    """json5
    {
      // Replacing with the value of a variable captured previously.
      "user_id": "$user_id",
      "item_name": "Watermelon"
    }
    """
    # Variable interpolation works also with body from file.

    Then I should have response with body
    """json5
    {
     "id":"<ignore-diff>",
     "created_at":"<ignore-diff>","updated_at": "<ignore-diff>",
     "prefixed_user_id":"static_prefix::$user_id",
     "prefixed_user": "static_prefix::$user"
    }
    """