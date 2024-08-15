/// <reference path="../pb_data/types.d.ts" />
migrate((db) => {
  const dao = new Dao(db)
  const collection = dao.findCollectionByNameOrId("oshm2acyheop48p")

  // remove
  collection.schema.removeField("ahe444to")

  // add
  collection.schema.addField(new SchemaField({
    "system": false,
    "id": "vhdixo3a",
    "name": "plan",
    "type": "relation",
    "required": false,
    "presentable": false,
    "unique": false,
    "options": {
      "collectionId": "il9hs1z49idlhle",
      "cascadeDelete": false,
      "minSelect": null,
      "maxSelect": 1,
      "displayFields": null
    }
  }))

  // add
  collection.schema.addField(new SchemaField({
    "system": false,
    "id": "23mspafd",
    "name": "start_date",
    "type": "date",
    "required": false,
    "presentable": false,
    "unique": false,
    "options": {
      "min": "",
      "max": ""
    }
  }))

  // add
  collection.schema.addField(new SchemaField({
    "system": false,
    "id": "woqae0kt",
    "name": "end_date",
    "type": "date",
    "required": false,
    "presentable": false,
    "unique": false,
    "options": {
      "min": "",
      "max": ""
    }
  }))

  // add
  collection.schema.addField(new SchemaField({
    "system": false,
    "id": "t9lqjlso",
    "name": "connection",
    "type": "json",
    "required": false,
    "presentable": false,
    "unique": false,
    "options": {
      "maxSize": 2000000
    }
  }))

  return dao.saveCollection(collection)
}, (db) => {
  const dao = new Dao(db)
  const collection = dao.findCollectionByNameOrId("oshm2acyheop48p")

  // add
  collection.schema.addField(new SchemaField({
    "system": false,
    "id": "ahe444to",
    "name": "connection_string",
    "type": "text",
    "required": false,
    "presentable": false,
    "unique": false,
    "options": {
      "min": null,
      "max": null,
      "pattern": ""
    }
  }))

  // remove
  collection.schema.removeField("vhdixo3a")

  // remove
  collection.schema.removeField("23mspafd")

  // remove
  collection.schema.removeField("woqae0kt")

  // remove
  collection.schema.removeField("t9lqjlso")

  return dao.saveCollection(collection)
})
