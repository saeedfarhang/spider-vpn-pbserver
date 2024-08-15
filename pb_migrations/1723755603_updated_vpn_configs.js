/// <reference path="../pb_data/types.d.ts" />
migrate((db) => {
  const dao = new Dao(db)
  const collection = dao.findCollectionByNameOrId("oshm2acyheop48p")

  collection.viewRule = "@request.auth.id = user.id"
  collection.createRule = "@request.auth.id = user.id"

  return dao.saveCollection(collection)
}, (db) => {
  const dao = new Dao(db)
  const collection = dao.findCollectionByNameOrId("oshm2acyheop48p")

  collection.viewRule = null
  collection.createRule = null

  return dao.saveCollection(collection)
})
