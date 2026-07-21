library "gender-boolean-parameter"
using FHIR version '4.0.0'
include FHIRHelpers version '4.0.0'

parameter "Male" Boolean

context Patient

define InInitialPopulation:
  if "Male" then Patient.gender = 'male' else Patient.gender = 'female'
