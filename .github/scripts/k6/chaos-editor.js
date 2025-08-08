import http from 'k6/http';
import exec from 'k6/execution';
import { fail, check, sleep } from 'k6';
import { randomIntBetween, randomItem } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

// Configuration for editable fields by resource type
const editableFieldsByResourceType = {
  Patient: [
    {
      name: 'name.given',
      edit: (resource) => {
        if (resource.name && resource.name.length > 0) {
          const nameIndex = randomIntBetween(0, resource.name.length - 1);
          if (resource.name[nameIndex].given && resource.name[nameIndex].given.length > 0) {
            const givenIndex = randomIntBetween(0, resource.name[nameIndex].given.length - 1);
            resource.name[nameIndex].given[givenIndex] = `Modified-${Date.now()}`;
            return `Changed name.given to ${resource.name[nameIndex].given[givenIndex]}`;
          }
        }
        return null;
      }
    },
    {
      name: 'gender',
      edit: (resource) => {
        const genders = ['male', 'female', 'other', 'unknown'];
        const currentGender = resource.gender;
        // Filter out the current gender to ensure we pick a different one
        const availableGenders = genders.filter(g => g !== currentGender);
        resource.gender = randomItem(availableGenders);
        return `Changed gender from ${currentGender} to ${resource.gender}`;
      }
    },
    {
      name: 'birthDate',
      edit: (resource) => {
        // Generate a random date in the past
        const year = randomIntBetween(1920, 2010);
        const month = randomIntBetween(1, 12).toString().padStart(2, '0');
        const day = randomIntBetween(1, 28).toString().padStart(2, '0');
        const newDate = `${year}-${month}-${day}`;
        const oldDate = resource.birthDate;
        resource.birthDate = newDate;
        return `Changed birthDate from ${oldDate} to ${newDate}`;
      }
    }
  ],
  Observation: [
    {
      name: 'status',
      edit: (resource) => {
        const statuses = ['registered', 'preliminary', 'final', 'amended', 'corrected', 'cancelled', 'entered-in-error', 'unknown'];
        const currentStatus = resource.status;
        const availableStatuses = statuses.filter(s => s !== currentStatus);
        resource.status = randomItem(availableStatuses);
        return `Changed status from ${currentStatus} to ${resource.status}`;
      }
    },
    {
      name: 'valueQuantity',
      edit: (resource) => {
        if (resource.valueQuantity) {
          const oldValue = resource.valueQuantity.value;
          // Modify the value by +/- 10%
          const change = oldValue * (randomIntBetween(-10, 10) / 100);
          resource.valueQuantity.value = +(oldValue + change).toFixed(2);
          return `Changed valueQuantity.value from ${oldValue} to ${resource.valueQuantity.value}`;
        }
        return null;
      }
    }
  ],
  Condition: [
    {
      name: 'clinicalStatus',
      edit: (resource) => {
        const statuses = [
          { coding: [{ system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'active' }] },
          { coding: [{ system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'recurrence' }] },
          { coding: [{ system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'relapse' }] },
          { coding: [{ system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'inactive' }] },
          { coding: [{ system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'remission' }] },
          { coding: [{ system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'resolved' }] }
        ];
        
        const currentCode = resource.clinicalStatus?.coding?.[0]?.code;
        const availableStatuses = statuses.filter(s => s.coding[0].code !== currentCode);
        const newStatus = randomItem(availableStatuses);
        
        resource.clinicalStatus = newStatus;
        return `Changed clinicalStatus from ${currentCode} to ${newStatus.coding[0].code}`;
      }
    },
    {
      name: 'severity',
      edit: (resource) => {
        const severities = [
          { coding: [{ system: 'http://terminology.hl7.org/CodeSystem/condition-severity', code: 'mild' }] },
          { coding: [{ system: 'http://terminology.hl7.org/CodeSystem/condition-severity', code: 'moderate' }] },
          { coding: [{ system: 'http://terminology.hl7.org/CodeSystem/condition-severity', code: 'severe' }] }
        ];
        
        const currentCode = resource.severity?.coding?.[0]?.code;
        const availableSeverities = severities.filter(s => s.coding[0].code !== currentCode);
        const newSeverity = randomItem(availableSeverities);
        
        resource.severity = newSeverity;
        return `Changed severity from ${currentCode} to ${newSeverity.coding[0].code}`;
      }
    }
  ],
  Medication: [
    {
      name: 'status',
      edit: (resource) => {
        const statuses = ['active', 'inactive', 'entered-in-error'];
        const currentStatus = resource.status;
        const availableStatuses = statuses.filter(s => s !== currentStatus);
        resource.status = randomItem(availableStatuses);
        return `Changed status from ${currentStatus} to ${resource.status}`;
      }
    }
  ]
};

// Default parameters
const params = {
  baseUrl: __ENV.BASE_URL || 'http://localhost:8080/fhir',
  duration: __ENV.DURATION || '30s' // Default duration is 30 seconds
};

// k6 options
export const options = {
  vus: 8, // Number of virtual users
  duration: params.duration, // Test duration
};

// Track the start time of the test
const startTime = new Date();

export default function() {
  // Get a list of all resource types we can edit
  const supportedResourceTypes = Object.keys(editableFieldsByResourceType);
  
  // Randomly select a resource type
  const resourceType = randomItem(supportedResourceTypes);
  console.log(`Selected resource type: ${resourceType}`);
  
  // First, get a list of resources of this type
  const searchUrl = `${params.baseUrl}/${resourceType}?_count=100`;
  console.log(`Searching for resources: ${searchUrl}`);
  
  const searchResponse = http.get(searchUrl, {
    headers: {
      'Accept': 'application/fhir+json'
    }
  });
  
  if (searchResponse.status !== 200) {
    console.log(`Failed to search for resources: ${searchResponse.status}`);
    sleep(1);
    return;
  }
  
  let bundle;
  try {
    bundle = searchResponse.json();
  } catch (e) {
    console.log(`Failed to parse search response: ${e.message}`);
    sleep(1);
    return;
  }
  
  if (!bundle.entry || bundle.entry.length === 0) {
    console.log(`No ${resourceType} resources found`);
    sleep(1);
    return;
  }
  
  // Select a random resource from the bundle
  const randomEntry = randomItem(bundle.entry);
  const resource = randomEntry.resource;
  const resourceId = resource.id;
  
  console.log(`Selected ${resourceType}/${resourceId} for editing`);
  
  // Get the editable fields for this resource type
  const editableFields = editableFieldsByResourceType[resourceType];
  
  // Select a random field to edit
  let editResult = null;
  let attempts = 0;
  const maxAttempts = editableFields.length * 2;
  
  while (!editResult && attempts < maxAttempts) {
    const field = randomItem(editableFields);
    console.log(`Attempting to edit field: ${field.name}`);
    
    editResult = field.edit(resource);
    attempts++;
    
    if (!editResult) {
      console.log(`Field ${field.name} could not be edited, trying another field...`);
    }
  }
  
  if (!editResult) {
    console.log(`Could not find any editable field in the resource after ${attempts} attempts`);
    sleep(1);
    return;
  }
  
  console.log(`Successfully edited resource: ${editResult}`);
  
  // Update the resource
  const updateResponse = http.put(`${params.baseUrl}/${resourceType}/${resourceId}`, JSON.stringify(resource), {
    headers: {
      'Accept': 'application/fhir+json',
      'Content-Type': 'application/fhir+json'
    }
  });
  
  check(updateResponse, {
    'Resource updated successfully': (r) => r.status === 200 || r.status === 201,
  });
  
  if (updateResponse.status !== 200 && updateResponse.status !== 201) {
    console.log(`Failed to update resource: ${updateResponse.status} ${updateResponse.body}`);
    sleep(1);
    return;
  }
  
  console.log(`Resource ${resourceType}/${resourceId} successfully updated`);
  
  // Add a small delay to avoid overwhelming the server
  sleep(0.5);
}
