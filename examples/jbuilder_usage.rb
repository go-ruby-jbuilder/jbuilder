# frozen_string_literal: true

require "jbuilder"

# Jbuilder builds JSON with a method_missing DSL: json.<name> sets a key,
# json.<name> do ... end nests an object, and Jbuilder.encode returns the
# compact JSON string.

# Scalars plus a nested object block.
puts Jbuilder.encode { |json|
  json.name "Widget"
  json.price 9.99
  json.in_stock true
  json.author do
    json.first "Ada"
    json.last "Lovelace"
  end
}
# => {"name":"Widget","price":9.99,"in_stock":true,"author":{"first":"Ada","last":"Lovelace"}}

# array! turns the whole document into a JSON array, mapping a block over a
# collection.
puts Jbuilder.encode { |json|
  json.array!([1, 2, 3]) { |n| json.square n * n }
}
# => [{"square":1},{"square":4},{"square":9}]

# key_format! renames keys, extract! copies attributes, merge! folds in a Hash,
# and ignore_nil! drops nil-valued keys.
puts Jbuilder.encode { |json|
  json.key_format! camelize: :lower
  json.extract!({ first_name: "Grace", last_name: "Hopper" }, :first_name, :last_name)
  json.merge!({ "rank" => "Rear Admiral" })
  json.ignore_nil!
  json.middle_name nil
}
# => {"firstName":"Grace","lastName":"Hopper","rank":"Rear Admiral"}

# Jbuilder.new yields a reusable builder; target! renders it.
builder = Jbuilder.new { |json| json.id 7 }
puts builder.target!
# => {"id":7}
