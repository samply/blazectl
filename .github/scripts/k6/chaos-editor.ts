import {check, sleep} from "k6";
import {randomIntBetween, randomItem} from "https://jslib.k6.io/k6-utils/1.2.0/index.js";
import http from "k6/http";
import {Options} from "k6/options";

// Type definitions
type ResourceType = 'Patient' | 'Observation' | 'Condition' | 'Medication';

interface EditableField {
    name: string;
    edit: (resource: any) => string | null;
}

interface EditableFieldsByResourceType {
    [key: ResourceType]: EditableField[];
}

interface FhirResource {
    id: string;
    meta?: {
        versionId?: string;
    };

    [key: string]: any;
}

interface FhirBundle {
    entry?: Array<{
        resource: FhirResource;
    }>;

    [key: string]: any;
}

interface TestParams {
    baseUrl: string;
    duration: string;
}

interface ResourcesMap {
    [key: ResourceType]: Array<string>;
}

// Configuration for editable fields by resource type
const editableFieldsByResourceType: EditableFieldsByResourceType = {
    Patient: [
        {
            name: 'name.given',
            edit: (resource: any): string | null => {
                if (resource.name?.length > 0) {
                    const nameIndex = randomIntBetween(0, resource.name.length - 1);
                    if (resource.name[nameIndex].given?.length > 0) {
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
            edit: (resource: any): string => {
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
            edit: (resource: any): string => {
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
            edit: (resource: any): string => {
                const statuses = ['registered', 'preliminary', 'final', 'amended', 'corrected', 'cancelled', 'entered-in-error', 'unknown'];
                const currentStatus = resource.status;
                const availableStatuses = statuses.filter(s => s !== currentStatus);
                resource.status = randomItem(availableStatuses);
                return `Changed status from ${currentStatus} to ${resource.status}`;
            }
        },
        {
            name: 'valueQuantity',
            edit: (resource: any): string | null => {
                if (resource.valueQuantity) {
                    const oldValue = resource.valueQuantity.value;
                    // Modify the value by +/- 10%
                    const change = oldValue * (randomIntBetween(-10, 10) / 100);
                    resource.valueQuantity.value = Number((oldValue + change).toFixed(2));
                    return `Changed valueQuantity.value from ${oldValue} to ${resource.valueQuantity.value}`;
                }
                return null;
            }
        }
    ],
    Condition: [
        {
            name: 'clinicalStatus',
            edit: (resource: any): string => {
                const statuses = [
                    {coding: [{system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'active'}]},
                    {
                        coding: [{
                            system: 'http://terminology.hl7.org/CodeSystem/condition-clinical',
                            code: 'recurrence'
                        }]
                    },
                    {coding: [{system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'relapse'}]},
                    {coding: [{system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'inactive'}]},
                    {coding: [{system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'remission'}]},
                    {coding: [{system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'resolved'}]}
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
            edit: (resource: any): string => {
                const severities = [
                    {coding: [{system: 'http://terminology.hl7.org/CodeSystem/condition-severity', code: 'mild'}]},
                    {coding: [{system: 'http://terminology.hl7.org/CodeSystem/condition-severity', code: 'moderate'}]},
                    {coding: [{system: 'http://terminology.hl7.org/CodeSystem/condition-severity', code: 'severe'}]}
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
            edit: (resource: any): string => {
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
const params: TestParams = {
    baseUrl: __ENV.BASE_URL || 'http://localhost:8080/fhir',
    duration: __ENV.DURATION || '30s', // The default duration is 30 seconds
    vus: __ENV.VUS || 8, // The default duration is 30 seconds
};

// k6 options
export let options: Options = {
    vus: params.vus,
    duration: params.duration,
};

// Setup function to download resources for all types once
const MEDIA_TYPE_FHIR = 'application/fhir+json';

export function setup(): ResourcesMap {
    console.log('Setting up resources cache...');
    const resourcesMap: ResourcesMap = {};
    const supportedResourceTypes = Object.keys(editableFieldsByResourceType) as ResourceType[];

    for (const resourceType of supportedResourceTypes) {
        let bundle;
        try {
            bundle = searchResources(resourceType);
        } catch (e) {
            console.error(e instanceof Error ? e.message : String(e));
            sleep(1);
            continue;
        }

        if (!bundle.entry || bundle.entry.length === 0) {
            console.warn(`No ${resourceType} resources found`);
            continue;
        }

        resourcesMap[resourceType] = bundle.entry.map(e => (e.resource.id));
        console.log(`Cached ${resourcesMap[resourceType].length} ${resourceType} resources`);
    }

    return resourcesMap;
}

function searchResources(resourceType: ResourceType): FhirBundle {
    console.log(`Downloading resources for type: ${resourceType}`);
    const searchUrl = `${params.baseUrl}/${resourceType}?_count=1000`;

    const searchResponse = http.get(searchUrl, {
        headers: {
            'Accept': MEDIA_TYPE_FHIR
        }
    });

    if (searchResponse.status !== 200) {
        throw Error(`Failed to search for ${resourceType} resources: ${searchResponse.status}`);
    }

    try {
        return searchResponse.json();
    } catch (e) {
        throw Error(`Failed to parse ${resourceType} search response: ${e instanceof Error ? e.message : String(e)}`);
    }
}

export default function (data: ResourcesMap): void {
    // Get a list of all resource types we can edit that have cached resources
    const availableResourceTypes = Object.keys(data).filter(
        type => data[type].length > 0
    ) as ResourceType[];

    if (availableResourceTypes.length === 0) {
        console.warn('No resources available in any resource type. Exiting.');
        sleep(1);
        return;
    }

    // Randomly select a resource from cached resources
    const resourceType = randomItem(availableResourceTypes);
    const resourceId = randomItem(data[resourceType]);

    let resource;
    try {
        resource = fetchResource(resourceType, resourceId)
    } catch (e) {
        console.warn(e instanceof Error ? e.message : String(e));
        sleep(1);
        return;
    }

    // Get the editable fields for this resource type
    const editableFields = editableFieldsByResourceType[resourceType];

    // Select a random field to edit
    const field = randomItem(editableFields);
    console.log(`Attempting to edit '${resourceType}/${resource.id}.${field.name}`);

    const editResult = field.edit(resource);
    if (editResult) {
        console.log(`Successfully edited resource: ${editResult}`);
    } else {
        console.warn(`Field ${field.name} could not be edited`);
        sleep(1);
        return;
    }

    const updateResponse = http.put(`${params.baseUrl}/${resourceType}/${resourceId}`, JSON.stringify(resource), {
        headers: {
            'Accept': MEDIA_TYPE_FHIR,
            'Content-Type': MEDIA_TYPE_FHIR,
            ...(resource.meta?.versionId && {'If-Match': `W/"${resource.meta.versionId}"`})
        }
    });

    if (!check(updateResponse, {
        'Resource updated successfully': (r) => r.status === 200,
    })) {
        console.error(`Failed to update resource: ${updateResponse.status} ${updateResponse.body}`);
        sleep(1);
        return;
    }

    console.log(`Resource ${resourceType}/${resourceId} successfully updated`);

    sleep(0.250);
}

function fetchResource(resourceType: ResourceType, resourceId: string): FhirResource {
    const url = `${params.baseUrl}/${resourceType}/${resourceId}`;

    const response = http.get(url, {
        headers: {
            'Accept': MEDIA_TYPE_FHIR
        }
    });

    if (response.status !== 200) {
        throw Error(`Failed to fetch resource ${resourceType}/${resourceId}: ${response.status}`) ;
    }

    try {
        return response.json();
    } catch (e) {
        throw Error(`Failed to parse ${resourceType} search response: ${e instanceof Error ? e.message : String(e)}`) ;
    }
}
