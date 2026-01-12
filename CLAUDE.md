# Itinerary Project Development Guide

## Iterative Development Loop
When implementing features for the Itinerary project, follow this strict procedure:

1. **Pick a component** we are going to work on
2. **Design the implementation**: Talk about the approach and hash out an implementation plan. Write this plan to `spec/components/<component>.md`
3. **Define the test suite**: Decide on the tests we need to verify the implementation is correct. Write this test plan to `spec/components/<component>-tests.md`
4. **Write the test suite**: Implement all tests defined in the test plan. User will review and may ask for more tests.
5. **Implement the feature**: Write the implementation to make the tests pass

## Git Commit Policy
* **Commit after each step** in the procedure above if there are changes to any files in that step
* Use descriptive commit messages that explain what was done in that step
* We will squash the history later, so frequent commits are encouraged
* This ensures any accidental deletions or mistakes can be recovered from easily

## Development Rules

### Dependency Management
* Avoid unneeded external dependencies
* Prefer the Go standard library where possible
* **Always ask before adding an external library**
* Any external library must be explicitly called out in the implementation plan

### Performance Requirements
* **NO IO IN THE SCHEDULER LOOP HOTPATH**
* The main scheduler loop must never perform I/O operations directly
* All I/O must be delegated to other goroutines
* This is a critical invariant that must never be violated

### Code Quality
* Write idiomatic Go code
* Use clear, descriptive variable and function names
* Add comments for complex logic
* Keep functions small and focused
